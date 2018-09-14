#include "queue.h"
#include "dtls.h"

#define ONE_YEAR 60 * 60 * 24 * 365

static inline int trivial_verify(int preverify_ok, X509_STORE_CTX *ctx) {
  (void)preverify_ok;
  (void)ctx;
  return 1;
}

void dtls_init() {
  OpenSSL_add_ssl_algorithms();
  SSL_load_error_strings();
  SSL_library_init();
}

SSL_CTX *dtls_build_ssl_context(dtls_cert_st *cert) {
  if (cert == NULL)
    return NULL;

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
  if (!ecdh)
    goto error;

  if (SSL_CTX_set_tmp_ecdh(ctx, ecdh) != 1)
    goto error;

  EC_KEY_free(ecdh);
#endif

  SSL_CTX_set_read_ahead(ctx, 1);
  SSL_CTX_set_verify(ctx, SSL_VERIFY_PEER | SSL_VERIFY_FAIL_IF_NO_PEER_CERT, trivial_verify);

  if (SSL_CTX_set_tlsext_use_srtp( ctx, "SRTP_AES128_CM_SHA1_32:SRTP_AES128_CM_SHA1_80") != 0) {
    goto error;
  }

  if (!SSL_CTX_use_certificate(ctx, cert->cert))
    goto error;

  if (!SSL_CTX_use_PrivateKey(ctx, cert->pkey) ||
      !SSL_CTX_check_private_key(ctx))
    goto error;

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

dtls_sess_st *dtls_build_session(SSL_CTX *ctx, bool is_offer) {
  dtls_sess_st *sess = (dtls_sess_st *)malloc(sizeof(dtls_sess_st));
  BIO *rbio = NULL;
  BIO *wbio = NULL;

  sess->state = is_offer;

  if (NULL == (sess->ssl = SSL_new(ctx)))
    goto error;

  if (NULL == (rbio = BIO_new(BIO_s_mem())))
    goto error;

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

ptrdiff_t dtls_flush(dtls_sess_st *sess, queue_st *queue) {
  if (sess->ssl == NULL)
    return -2;

  BIO *wbio = SSL_get_wbio(sess->ssl);
  size_t pending = BIO_ctrl_pending(wbio);
  if (pending > 0) {
    char *buf = malloc(pending);
    dtls_buffer_st *buffer = (dtls_buffer_st *)malloc(sizeof(dtls_buffer_st));
    buffer->size = BIO_read(wbio, buf, pending);
    buffer->data = realloc(buf, buffer->size);

    int err = queue_put(queue, buffer);
    if (err != 0)
      return 0;

    return buffer->size;
  }
  return 0;
}

static inline enum dtls_con_state
dtls_sess_get_state(const dtls_sess_st *sess) {
  return sess->state;
}

ptrdiff_t dtls_do_handshake(dtls_sess_st *sess, queue_st *queue, char *local, char *remote) {
  if (sess->ssl == NULL ||
     (dtls_sess_get_state(sess) != DTLS_CONSTATE_ACT &&
     dtls_sess_get_state(sess) != DTLS_CONSTATE_ACTPASS))
    return -2;

  if (dtls_sess_get_state(sess) == DTLS_CONSTATE_ACTPASS)
    sess->state = DTLS_CONSTATE_ACT;

  SSL_do_handshake(sess->ssl);
  return dtls_flush(sess, queue);
}

dtls_cert_st *dtls_build_certificate() {
  dtls_cert_st *cert = (dtls_cert_st *)malloc(sizeof(dtls_cert_st));

  static const int num_bits = 2048;

  BIGNUM *bne = BN_new();
  if (bne == NULL)
    goto error;

  if (!BN_set_word(bne, RSA_F4))
    goto error;

  RSA *rsa_key = RSA_new();
  if (rsa_key == NULL)
    goto error;

  if (!RSA_generate_key_ex(rsa_key, num_bits, bne, NULL))
    goto error;

  if ((cert->pkey = EVP_PKEY_new()) == NULL)
    goto error;

  if (!EVP_PKEY_assign_RSA(cert->pkey, rsa_key))
    goto error;

  rsa_key = NULL;
  if ((cert->cert = X509_new()) == NULL)
    goto error;

  X509_set_version(cert->cert, 2);
  ASN1_INTEGER_set(X509_get_serialNumber(cert->cert), 1000); // TODO
  X509_gmtime_adj(X509_get_notBefore(cert->cert), -1 * ONE_YEAR);
  X509_gmtime_adj(X509_get_notAfter(cert->cert), ONE_YEAR);
  if (!X509_set_pubkey(cert->cert, cert->pkey))
    goto error;

  X509_NAME *cert_name = cert_name = X509_get_subject_name(cert->cert);
  if (cert_name == NULL)
    goto error;

  const char *name = "pion-webrtc";
  X509_NAME_add_entry_by_txt(cert_name, "O", MBSTRING_ASC, (const char unsigned *)name, -1, -1, 0);
  X509_NAME_add_entry_by_txt(cert_name, "CN", MBSTRING_ASC, (const char unsigned *)name, -1, -1, 0);

  if (!X509_set_issuer_name(cert->cert, cert_name))
    goto error;

  if (!X509_sign(cert->cert, cert->pkey, EVP_sha1()))
    goto error;

  BN_free(bne);
  return cert;

error:
  if (bne)
    BN_free(bne);

  if (rsa_key && !cert && !cert->pkey)
    RSA_free(rsa_key);

  if (cert && cert->pkey)
    EVP_PKEY_free(cert->pkey);

  if (cert && cert->cert)
    X509_free(cert->cert);

  return NULL;
}

void dtls_session_cleanup(SSL_CTX *ctx, dtls_sess_st *sess, dtls_cert_st *cert) {
  if (sess) {
    if (sess->ssl != NULL) {
      SSL_free(sess->ssl);
      sess->ssl = NULL;
    }
    free(sess);
  }

  if (ctx)
    SSL_CTX_free(ctx);

  if (cert) {
    if (cert->cert)
      X509_free(cert->cert);

    if (cert->pkey)
      EVP_PKEY_free(cert->pkey);

    free(cert);
  }
}

dtls_buffer_st *dtls_handle_incoming(dtls_sess_st *sess, queue_st *queue, void *buf, int len) {
  if (sess->ssl == NULL)
    return NULL;

  BIO *rbio = SSL_get_rbio(sess->ssl);

  if (sess->state == DTLS_CONSTATE_ACTPASS) {
    sess->state = DTLS_CONSTATE_PASS;
    SSL_set_accept_state(sess->ssl);
  }

  int decrypted_len = 0;
  void *decrypted = malloc(len);
  BIO_write(rbio, buf, len);
  decrypted_len = SSL_read(sess->ssl, decrypted, len);

  if ((decrypted_len < 0) && SSL_get_error(sess->ssl, decrypted_len) == SSL_ERROR_SSL) {
    fprintf(stderr, "DTLS failure occurred on dtls session %p due to reason '%s'\n", sess, ERR_reason_error_string(ERR_get_error()));
    free(decrypted);
    return NULL;
  }

  dtls_flush(sess, queue);

  if (SSL_is_init_finished(sess->ssl))
    sess->type = DTLS_CONTYPE_EXISTING;

  dtls_buffer_st *ret;
  if (decrypted_len > 1) {
    ret = (dtls_buffer_st *)malloc(sizeof(dtls_buffer_st));
    ret->data = decrypted;
    ret->size = decrypted_len;
  } else {
    free(decrypted);
    return NULL;
  }

  return ret;
}

bool dtls_handle_outgoing(dtls_sess_st *sess, queue_st *queue, void *buf, int len) {
  if (sess->ssl == NULL)
    return false;

  int written = SSL_write(sess->ssl, buf, len);
  if (written != len) {
    if (SSL_get_error(sess->ssl, written) == SSL_ERROR_SSL) {
      fprintf(stderr, "DTLS failure occurred on dtls session %p due to reason '%s'\n", sess, ERR_reason_error_string(ERR_get_error()));
    }
    return false;
  }

  dtls_flush(sess, queue);

  return true;
}

char *dtls_certificate_fingerprint(dtls_cert_st *cert) {
  if (cert == NULL)
    return NULL;

  unsigned int size;
  unsigned char fingerprint[EVP_MAX_MD_SIZE];
  if (X509_digest(cert->cert, EVP_sha256(), (unsigned char *)fingerprint,
                  &size) == 0) {
    return NULL;
  }

  char *hex_fingeprint = malloc(sizeof(char) * 160);
  char *curr = hex_fingeprint;
  unsigned int i = 0;
  for (i = 0; i < size; i++) {
    sprintf(curr, "%.2X:", fingerprint[i]);
    curr += 3;
  }
  *(curr - 1) = '\0';
  return hex_fingeprint;
}

dtls_cert_pair *dtls_get_certpair(dtls_sess_st *sess) {
  if (sess->type == DTLS_CONTYPE_EXISTING) {
    unsigned char dtls_buffer[SRTP_MASTER_KEY_KEY_LEN * 2 + SRTP_MASTER_KEY_SALT_LEN * 2];

    const char *label = "EXTRACTOR-dtls_srtp";
    if (!SSL_export_keying_material(sess->ssl, dtls_buffer, sizeof(dtls_buffer), label, strlen(label), NULL, 0, 0)) {
      fprintf(stderr, "SSL_export_keying_material failed");
      return NULL;
    }

    size_t offset = 0;
    dtls_cert_pair *ret = malloc(sizeof(dtls_cert_pair));
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
