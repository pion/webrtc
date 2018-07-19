#include "dtls.h"

#define ONE_YEAR 60 * 60 * 24 * 365

// strdup is POSIX extension
char *dtls_strdup(char *src) {
  char *str;
  size_t len = strlen(src) + 1;

  str = malloc(len);
  if (str) {
    memcpy(str, src, len);
  }
  return str;
}

bool openssl_global_init() {
  OpenSSL_add_ssl_algorithms();
  SSL_load_error_strings();
  return SSL_library_init();
}

int dtls_trivial_verify_callback(int preverify_ok, X509_STORE_CTX *ctx) {
  (void)preverify_ok;
  (void)ctx;
  return 1;
}

SSL_CTX *dtls_build_sslctx(tlscfg *cfg) {
  if (cfg == NULL) {
    return NULL;
  }

#if (OPENSSL_VERSION_NUMBER >= 0x10100000L)
  SSL_CTX *ctx = SSL_CTX_new(DTLS_method());
#elif (OPENSSL_VERSION_NUMBER >= 0x10001000L)
  SSL_CTX *ctx = SSL_CTX_new(DTLSv1_method());
#else
#error "Unsupported OpenSSL Version"
#endif

#if (OPENSSL_VERSION_NUMBER >= 0x10002000L)
  SSL_CTX_set_ecdh_auto(ctx, true);
#else
  EC_KEY *ecdh = EC_KEY_new_by_curve_name(NID_X9_62_prime256v1);

  if (!ecdh) {
    goto error;
  }

  if (SSL_CTX_set_tmp_ecdh(ctx, ecdh) != 1) {
    goto error;
  }
  EC_KEY_free(ecdh);
#endif

  SSL_CTX_set_read_ahead(ctx, 1);
  SSL_CTX_set_verify(ctx, SSL_VERIFY_PEER | SSL_VERIFY_FAIL_IF_NO_PEER_CERT, dtls_trivial_verify_callback);

  if (SSL_CTX_set_tlsext_use_srtp(ctx, "SRTP_AES128_CM_SHA1_32:SRTP_AES128_CM_SHA1_80") != 0) {
    goto error;
  }

  if (!SSL_CTX_use_certificate(ctx, cfg->cert)) {
    goto error;
  }

  if (!SSL_CTX_use_PrivateKey(ctx, cfg->pkey) || !SSL_CTX_check_private_key(ctx)) {
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

dtls_sess *dtls_build_session(SSL_CTX *sslcfg, bool is_server) {
  dtls_sess *sess = (dtls_sess *)calloc(1, sizeof(dtls_sess));
  BIO *rbio = NULL;
  BIO *wbio = NULL;

  sess->state = is_server;

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

  return sess;

error:
  if (sess->ssl != NULL) {
    SSL_free(sess->ssl);
    sess->ssl = NULL;
  }
  free(sess);
  return NULL;
}

extern void go_handle_sendto(const char *local, const char *remote, char *buf, int len);
ptrdiff_t dtls_sess_send_pending(dtls_sess *sess, char *local, char *remote) {
  if (sess->ssl == NULL) {
    return -2;
  }
  BIO *wbio = SSL_get_wbio(sess->ssl);
  size_t pending = BIO_ctrl_pending(wbio);
  size_t len = 0;
  if (pending > 0) {
    char *buf = malloc(pending);
    len = BIO_read(wbio, buf, pending);
    buf = realloc(buf, len);

    go_handle_sendto(local, remote, buf, len);
    return len;
  }
  return 0;
}

static inline enum dtls_con_state dtls_sess_get_state(const dtls_sess *sess) { return sess->state; }

ptrdiff_t dtls_do_handshake(dtls_sess *sess, char *local, char *remote) {
  if (sess->ssl == NULL ||
      (dtls_sess_get_state(sess) != DTLS_CONSTATE_ACT && dtls_sess_get_state(sess) != DTLS_CONSTATE_ACTPASS)) {
    return -2;
  }
  if (dtls_sess_get_state(sess) == DTLS_CONSTATE_ACTPASS) {
    sess->state = DTLS_CONSTATE_ACT;
  }
  SSL_do_handshake(sess->ssl);
  return dtls_sess_send_pending(sess, local, remote);
}

tlscfg *dtls_build_tlscfg() {
  tlscfg *cfg = (tlscfg *)calloc(1, sizeof(tlscfg));

  static const int num_bits = 2048;

  BIGNUM *bne = BN_new();
  if (bne == NULL) {
    goto error;
  }

  if (!BN_set_word(bne, RSA_F4)) {
    goto error;
  }

  RSA *rsa_key = RSA_new();
  if (rsa_key == NULL) {
    goto error;
  }

  if (!RSA_generate_key_ex(rsa_key, num_bits, bne, NULL)) {
    goto error;
  }

  if ((cfg->pkey = EVP_PKEY_new()) == NULL) {
    goto error;
  }

  if (!EVP_PKEY_assign_RSA(cfg->pkey, rsa_key)) {
    goto error;
  }

  rsa_key = NULL;
  if ((cfg->cert = X509_new()) == NULL) {
    goto error;
  }

  X509_set_version(cfg->cert, 2);
  ASN1_INTEGER_set(X509_get_serialNumber(cfg->cert), 1000); // TODO
  X509_gmtime_adj(X509_get_notBefore(cfg->cert), -1 * ONE_YEAR);
  X509_gmtime_adj(X509_get_notAfter(cfg->cert), ONE_YEAR);
  if (!X509_set_pubkey(cfg->cert, cfg->pkey)) {
    goto error;
  }

  X509_NAME *cert_name = cert_name = X509_get_subject_name(cfg->cert);
  if (cert_name == NULL) {
    goto error;
  }

  const char *name = "pion-webrtc";
  X509_NAME_add_entry_by_txt(cert_name, "O", MBSTRING_ASC, (const char unsigned *)name, -1, -1, 0);
  X509_NAME_add_entry_by_txt(cert_name, "CN", MBSTRING_ASC, (const char unsigned *)name, -1, -1, 0);

  if (!X509_set_issuer_name(cfg->cert, cert_name)) {
    goto error;
  }

  if (!X509_sign(cfg->cert, cfg->pkey, EVP_sha1())) {
    goto error;
  }

  BN_free(bne);
  return cfg;

error:
  if (bne) {
    BN_free(bne);
  }
  if (rsa_key && !cfg && !cfg->pkey) {
    RSA_free(rsa_key);
  }
  if (cfg && cfg->pkey) {
    EVP_PKEY_free(cfg->pkey);
  }
  if (cfg && cfg->cert) {
    X509_free(cfg->cert);
  }
  return NULL;
}


void dtls_session_cleanup(SSL_CTX *ssl_ctx, dtls_sess *dtls_session) {
  if (dtls_session) {
    if (dtls_session->ssl != NULL) {
      SSL_free(dtls_session->ssl);
      dtls_session->ssl = NULL;
    }

    free(dtls_session);
  }

  if (ssl_ctx) {
    SSL_CTX_free(ssl_ctx);
  }
}

void dtls_tlscfg_cleanup(tlscfg *cfg) {
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

dtls_decrypted *dtls_handle_incoming(dtls_sess *sess, void *buf, int len, char *local, char *remote) {
  if (sess->ssl == NULL) {
    return NULL;
  }

  int decrypted_len = 0;
  void *decrypted = calloc(1, len);
  dtls_decrypted *ret = NULL;

  BIO *rbio = SSL_get_rbio(sess->ssl);

  if (sess->state == DTLS_CONSTATE_ACTPASS) {
    sess->state = DTLS_CONSTATE_PASS;
    SSL_set_accept_state(sess->ssl);
  }

  dtls_sess_send_pending(sess, local, remote);

  BIO_write(rbio, buf, len);
  decrypted_len = SSL_read(sess->ssl, decrypted, len);

  if ((decrypted_len < 0) && SSL_get_error(sess->ssl, decrypted_len) == SSL_ERROR_SSL) {
     fprintf(stderr, "DTLS failure occurred on dtls session %p due to reason '%s'\n", sess,
             ERR_reason_error_string(ERR_get_error()));
     free(decrypted);
     return ret;
  }

  dtls_sess_send_pending(sess, local, remote);

  if (SSL_is_init_finished(sess->ssl)) {
    sess->type = DTLS_CONTYPE_EXISTING;
  }

  if (decrypted_len > 1) {
    ret = (dtls_decrypted *)calloc(1, sizeof(dtls_decrypted));
    ret->buf = decrypted;
    ret->len = decrypted_len;
  } else {
    free(decrypted);
  }

  return ret;
}

bool dtls_handle_outgoing(dtls_sess *sess, void *buf, int len, char *local, char *remote) {
  if (sess->ssl == NULL) {
    return false;
  }

  int written = SSL_write(sess->ssl, buf, len);
  if (written != len) {
    if (SSL_get_error(sess->ssl, written) == SSL_ERROR_SSL) {
      fprintf(stderr, "DTLS failure occurred on dtls session %p due to reason '%s'\n", sess,
          ERR_reason_error_string(ERR_get_error()));
    }
    return false;
  }

  dtls_sess_send_pending(sess, local, remote);

  return true;
}

char *dtls_tlscfg_fingerprint(tlscfg *cfg) {
  if (cfg == NULL) {
    return NULL;
  }

  unsigned int size;
  unsigned char fingerprint[EVP_MAX_MD_SIZE];
  if (X509_digest(cfg->cert, EVP_sha256(), (unsigned char *)fingerprint, &size) == 0) {
    return NULL;
  }

  char *hex_fingeprint = calloc(1, sizeof(char) * 160);
  char *curr = hex_fingeprint;
  unsigned int i = 0;
  for (i = 0; i < size; i++) {
    sprintf(curr, "%.2X:", fingerprint[i]);
    curr += 3;
  }
  *(curr - 1) = '\0';
  return hex_fingeprint;
}

dtls_cert_pair *dtls_get_certpair(dtls_sess *sess) {
  if (sess->type == DTLS_CONTYPE_EXISTING) {
    unsigned char dtls_buffer[SRTP_MASTER_KEY_KEY_LEN * 2 + SRTP_MASTER_KEY_SALT_LEN * 2];

    const char *label = "EXTRACTOR-dtls_srtp";
    if (!SSL_export_keying_material(sess->ssl, dtls_buffer, sizeof(dtls_buffer), label, strlen(label), NULL, 0, 0)) {
      fprintf(stderr, "SSL_export_keying_material failed");
      return NULL;
    }

    size_t offset = 0;
    dtls_cert_pair *ret = calloc(1, sizeof(dtls_cert_pair));
    ret->key_length = SRTP_MASTER_KEY_KEY_LEN + SRTP_MASTER_KEY_SALT_LEN;

    memcpy(&ret->client_write_key[0], &dtls_buffer[offset], SRTP_MASTER_KEY_KEY_LEN);
    offset += SRTP_MASTER_KEY_KEY_LEN;
    memcpy(&ret->server_write_key[0], &dtls_buffer[offset], SRTP_MASTER_KEY_KEY_LEN);
    offset += SRTP_MASTER_KEY_KEY_LEN;
    memcpy(&ret->client_write_key[SRTP_MASTER_KEY_KEY_LEN], &dtls_buffer[offset], SRTP_MASTER_KEY_SALT_LEN);
    offset += SRTP_MASTER_KEY_SALT_LEN;
    memcpy(&ret->server_write_key[SRTP_MASTER_KEY_KEY_LEN], &dtls_buffer[offset], SRTP_MASTER_KEY_SALT_LEN);

    switch (SSL_get_selected_srtp_profile(sess->ssl)->id) {
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
