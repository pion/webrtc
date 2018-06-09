#include "dtls.h"

#define ONE_YEAR 60*60*24*365

int dtls_trivial_verify_callback(int preverify_ok, X509_STORE_CTX *ctx) {
  (void) preverify_ok;
  (void) ctx;
  return 1;
}

SSL_CTX* dtls_ctx_init (tlscfg *cfg) {
  SSL_CTX* ctx = SSL_CTX_new(DTLS_method());

  SSL_CTX_set_read_ahead(ctx, 1);
  SSL_CTX_set_ecdh_auto(ctx, true);
  SSL_CTX_set_verify(ctx, SSL_VERIFY_PEER | SSL_VERIFY_FAIL_IF_NO_PEER_CERT, dtls_trivial_verify_callback);

  if (SSL_CTX_set_tlsext_use_srtp(ctx, "SRTP_AES128_CM_SHA1_32:SRTP_AES128_CM_SHA1_80") != 0) {
    goto error;
  }

  if (!SSL_CTX_use_certificate(ctx, cfg->cert)) {
    goto error;
  }

  if (!SSL_CTX_use_PrivateKey(ctx, cfg->pkey) ||
      !SSL_CTX_check_private_key(ctx)) {
    goto error;
  }

  if (!SSL_CTX_set_cipher_list(ctx, "HIGH:!aNULL:!MD5:!RC4")) {
    goto error;
  }

  return ctx;

error:
  if (ctx != NULL) {
    SSL_CTX_free(ctx);
  }
  return NULL;
}

dtls_sess* dtls_sess_new(SSL_CTX* sslcfg, int con_state) {
  dtls_sess* sess = (dtls_sess*)calloc(1, sizeof(dtls_sess));
  BIO* rbio = NULL;
  BIO* wbio = NULL;

  sess->state = con_state;

  if (NULL == (sess->ssl = SSL_new(sslcfg))) {
    goto error;
  }

  if (NULL == (rbio = BIO_new(BIO_s_mem()))) {
    goto error;
  }

  BIO_set_mem_eof_return(rbio, -1);

  if (NULL == (wbio = BIO_new(BIO_s_mem()))) {
    BIO_free(rbio);
    rbio = NULL;
    goto error;
  }

  BIO_set_mem_eof_return(wbio, -1);

  SSL_set_bio(sess->ssl, rbio, wbio);

  if (sess->state == DTLS_CONSTATE_PASS) {
    SSL_set_accept_state(sess->ssl);
  } else {
    SSL_set_connect_state(sess->ssl);
  }
  sess->type = DTLS_CONTYPE_NEW;

  pthread_mutex_init(&sess->lock, NULL);
  return sess;

error:
  if (sess->ssl != NULL) {
    SSL_free(sess->ssl);
    sess->ssl = NULL;
  }
  free(sess);
  return NULL;
}

void dtls_sess_free(dtls_sess* sess) {
  if (sess->ssl != NULL) {
    SSL_free(sess->ssl);
    sess->ssl = NULL;
  }
  pthread_mutex_destroy(&sess->lock);
  free(sess);
}

extern void go_handle_sendto(const char* src, const char* dst, char* buf,
			     int len);
ptrdiff_t dtls_sess_send_pending(dtls_sess* sess, const char* src,
				 const char* dst) {
  if (sess->ssl == NULL) {
    return -2;
  }
  BIO* wbio = SSL_get_wbio(sess->ssl);
  size_t pending = BIO_ctrl_pending(wbio);
  size_t len = 0;
  if (pending > 0) {
    char* buf = malloc(pending);
    len = BIO_read(wbio, buf, pending);
    buf = realloc(buf, len);

    go_handle_sendto(src, dst, buf, len);
    return len;
  }
  return 0;
}

ptrdiff_t dtls_sess_put_packet(dtls_sess* sess, const char* src,
			       const char* dst, const void* buf, size_t len) {
  if (sess->ssl == NULL) {
    return -1;
  }

  ptrdiff_t ret = 0;
  char dummy[len];

  pthread_mutex_lock(&sess->lock);
  pthread_mutex_unlock(&sess->lock);

  BIO* rbio = SSL_get_rbio(sess->ssl);

  if (sess->state == DTLS_CONSTATE_ACTPASS) {
    sess->state = DTLS_CONSTATE_PASS;
    SSL_set_accept_state(sess->ssl);
  }

  dtls_sess_send_pending(sess, src, dst);

  BIO_write(rbio, buf, len);
  ret = SSL_read(sess->ssl, dummy, len);

  if ((ret < 0) && SSL_get_error(sess->ssl, ret) == SSL_ERROR_SSL) {
    return ret;
  }

  ret = dtls_sess_send_pending(sess, src, dst);

  if (SSL_is_init_finished(sess->ssl)) {
    sess->type = DTLS_CONTYPE_EXISTING;
  }

  return ret;
}

static inline enum dtls_con_state dtls_sess_get_state(const dtls_sess* sess) {
  return sess->state;
}

ptrdiff_t dtls_do_handshake(dtls_sess* sess, const char* src, const char* dst) {
  if (sess->ssl == NULL ||
      (dtls_sess_get_state(sess) != DTLS_CONSTATE_ACT &&
       dtls_sess_get_state(sess) != DTLS_CONSTATE_ACTPASS)) {
    return -2;
  }
  if (dtls_sess_get_state(sess) == DTLS_CONSTATE_ACTPASS) {
    sess->state = DTLS_CONSTATE_ACT;
  }
  SSL_do_handshake(sess->ssl);
  pthread_mutex_lock(&sess->lock);
  ptrdiff_t ret = dtls_sess_send_pending(sess, src, dst);
  pthread_mutex_unlock(&sess->lock);
  return ret;
}

tlscfg* dtls_build_tlscfg() {
  tlscfg* cfg = (tlscfg*)calloc(1, sizeof(tlscfg));

  static const int num_bits = 2048;

  BIGNUM *bne = BN_new();
  if(bne == NULL) {
    goto error;
  }

  if(!BN_set_word(bne, RSA_F4)) {
    goto error;
  }

  RSA *rsa_key = RSA_new();
  if(rsa_key == NULL) {
    goto error;
  }

  if(!RSA_generate_key_ex(rsa_key, num_bits, bne, NULL)) {
    goto error;
  }

  if((cfg->pkey = EVP_PKEY_new()) == NULL) {
    goto error;
  }

  if(!EVP_PKEY_assign_RSA(cfg->pkey, rsa_key)) {
    goto error;
  }

  rsa_key = NULL;
  if((cfg->cert = X509_new()) == NULL) {
    goto error;
  }

  X509_set_version(cfg->cert, 2);
  ASN1_INTEGER_set(X509_get_serialNumber(cfg->cert), 1000); // TODO
  X509_gmtime_adj(X509_get_notBefore(cfg->cert), -1 * ONE_YEAR);
  X509_gmtime_adj(X509_get_notAfter(cfg->cert), ONE_YEAR);
  if(!X509_set_pubkey(cfg->cert, cfg->pkey)) {
    goto error;
  }

  X509_NAME *cert_name = cert_name = X509_get_subject_name(cfg->cert);
  if(cert_name == NULL) {
    goto error;
  }

  const char *name = "pion-webrtc";
  X509_NAME_add_entry_by_txt(cert_name, "O", MBSTRING_ASC, (const char unsigned *)name, -1, -1, 0);
  X509_NAME_add_entry_by_txt(cert_name, "CN", MBSTRING_ASC, (const char unsigned *)name, -1, -1, 0);

  if(!X509_set_issuer_name(cfg->cert, cert_name)) {
    goto error;
  }

  if(!X509_sign(cfg->cert, cfg->pkey, EVP_sha1())) {
    goto error;
  }

  BN_free(bne);
  return cfg;

error:
  if(bne)
    BN_free(bne);
  if(rsa_key && !cfg && !cfg->pkey)
    RSA_free(rsa_key);
  if(cfg && cfg->pkey)
    EVP_PKEY_free(cfg->pkey);
  if(cfg && cfg->cert)
    X509_free(cfg->cert);
  return NULL;
}

SSL_CTX* dtls_build_sslctx(tlscfg* cfg) {
  if (cfg == NULL) {
    return NULL;
  }

  return dtls_ctx_init(cfg);
}

dtls_sess* dtls_build_session(SSL_CTX* cfg, bool is_server) {
  return dtls_sess_new(cfg, is_server);
}

bool openssl_global_init() {
  OpenSSL_add_ssl_algorithms();
  SSL_load_error_strings();
  return SSL_library_init();
}

void dtls_session_cleanup(SSL_CTX* ssl_ctx, dtls_sess* dtls_session) {
  if (dtls_session) {
    dtls_sess_free(dtls_session);
  }
  if (ssl_ctx) {
    SSL_CTX_free(ssl_ctx);
  }
}

void dtls_tlscfg_cleanup(tlscfg* cfg) {
  if (cfg) {
    if (cfg->cert) {
      X509_free(cfg->cert);
    }
    if (cfg->pkey) {
      EVP_PKEY_free(cfg->pkey);
    }

    free(cfg);
  }
}



dtls_handle_incoming_return *dtls_handle_incoming(dtls_sess* sess, const char* src, const char* dst,
			  void* buf, int len) {
  ptrdiff_t stat = dtls_sess_put_packet(sess, src, dst, buf, len);
  if (SSL_get_error(sess->ssl, stat) == SSL_ERROR_SSL) {
    fprintf(stderr,
	"DTLS failure occurred on dtls session %p due to reason '%s'\n",
	sess, ERR_reason_error_string(ERR_get_error()));
    return NULL;
  }

  if (sess->type == DTLS_CONTYPE_EXISTING) {
    unsigned char dtls_buffer[SRTP_MASTER_KEY_KEY_LEN * 2 + SRTP_MASTER_KEY_SALT_LEN * 2];

    const char *label = "EXTRACTOR-dtls_srtp";
    if (!SSL_export_keying_material(sess->ssl, dtls_buffer, sizeof(dtls_buffer), label, strlen(label), NULL, 0, 0)) {
      fprintf(stderr, "SSL_export_keying_material failed");
      return NULL;
    }

    size_t offset = 0;
    dtls_handle_incoming_return *ret = calloc(1, sizeof(dtls_handle_incoming_return));
    ret->key_length = SRTP_MASTER_KEY_KEY_LEN + SRTP_MASTER_KEY_SALT_LEN;

    memcpy(&ret->client_write_key[0], &dtls_buffer[offset], SRTP_MASTER_KEY_KEY_LEN);
    offset += SRTP_MASTER_KEY_KEY_LEN;
    memcpy(&ret->server_write_key[0], &dtls_buffer[offset], SRTP_MASTER_KEY_KEY_LEN);
    offset += SRTP_MASTER_KEY_KEY_LEN;
    memcpy(&ret->client_write_key[SRTP_MASTER_KEY_KEY_LEN], &dtls_buffer[offset], SRTP_MASTER_KEY_SALT_LEN);
    offset += SRTP_MASTER_KEY_SALT_LEN;
    memcpy(&ret->server_write_key[SRTP_MASTER_KEY_KEY_LEN], &dtls_buffer[offset], SRTP_MASTER_KEY_SALT_LEN);

    switch(SSL_get_selected_srtp_profile(sess->ssl)->id) {
      case SRTP_AES128_CM_SHA1_80:
	memcpy(&ret->profile, "SRTP_AES128_CM_SHA1_80", strlen("SRTP_AES128_CM_SHA1_80"));
	break;
      case SRTP_AES128_CM_SHA1_32:
	memcpy(&ret->profile, "SRTP_AES128_CM_SHA1_32", strlen("SRTP_AES128_CM_SHA1_32"));
	break;
    }

    return ret;
  }

  return NULL;
}

char *dtls_tlscfg_fingerprint(tlscfg* cfg) {
  if (cfg == NULL) {
    return NULL;
  }

  unsigned int size;
  unsigned char fingerprint[EVP_MAX_MD_SIZE];
  if(X509_digest(cfg->cert, EVP_sha256(), (unsigned char *)fingerprint, &size) == 0) {
    return NULL;
  }

  char *hex_fingeprint = calloc(1, sizeof(char) * 160);
  char *curr = hex_fingeprint;
  unsigned int i = 0;
  for(i = 0; i < size; i++) {
    sprintf(curr, "%.2X:", fingerprint[i]);
    curr += 3;
  }
  *(curr-1) = '\0';
  return hex_fingeprint;
}
