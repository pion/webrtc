#include "dtls.h"

//recommended cipher suites.
const char cipherlist[] =
  "ECDHE-RSA-AES128-GCM-SHA256:"
  "ECDHE-ECDSA-AES128-GCM-SHA256:"
  "ECDHE-RSA-AES256-GCM-SHA384:"
  "ECDHE-ECDSA-AES256-GCM-SHA384:"
  "DHE-RSA-AES128-GCM-SHA256:"
  "kEDH+AESGCM:"
  "ECDHE-RSA-AES128-SHA256:"
  "ECDHE-ECDSA-AES128-SHA256:"
  "ECDHE-RSA-AES128-SHA:"
  "ECDHE-ECDSA-AES128-SHA:"
  "ECDHE-RSA-AES256-SHA384:"
  "ECDHE-ECDSA-AES256-SHA384:"
  "ECDHE-RSA-AES256-SHA:"
  "ECDHE-ECDSA-AES256-SHA:"
  "DHE-RSA-AES128-SHA256:"
  "DHE-RSA-AES128-SHA:"
  "DHE-RSA-AES256-SHA256:"
  "DHE-RSA-AES256-SHA:"
  "!aNULL:!eNULL:!EXPORT:!DSS:!DES:!RC4:!3DES:!MD5:!PSK";

static inline bool str_isempty(const char* str) {
  return ((str == NULL) || (str[0] == '\0'));
}

static inline const char* str_nullforempty(const char* str) {
  return (str_isempty(str) ? NULL : str);
}

static inline BIO* dtls_sess_get_rbio(dtls_sess* sess) {
  return SSL_get_rbio(sess->ssl);
}
static inline BIO* dtls_sess_get_wbio(dtls_sess* sess) {
  return SSL_get_wbio(sess->ssl);
}

srtp_key_material* srtp_get_key_material(dtls_sess* sess);
void key_material_free(srtp_key_material* km);

static inline void dtls_sess_set_state(dtls_sess* sess,
				       enum dtls_con_state state) {
  sess->state = state;
}

static inline enum dtls_con_state dtls_sess_get_state(const dtls_sess* sess) {
  return sess->state;
}

static inline void srtp_key_material_extract(const srtp_key_material* km,
					     srtp_key_ptrs* ptrs) {
  if (km->ispassive == DTLS_CONSTATE_ACT) {
    ptrs->localkey = (km->material);
    ptrs->remotekey = ptrs->localkey + MASTER_KEY_LEN;
    ptrs->localsalt = ptrs->remotekey + MASTER_KEY_LEN;
    ptrs->remotesalt = ptrs->localsalt + MASTER_SALT_LEN;
  } else {
    ptrs->remotekey = (km->material);
    ptrs->localkey = ptrs->remotekey + MASTER_KEY_LEN;
    ptrs->remotesalt = ptrs->localkey + MASTER_KEY_LEN;
    ptrs->localsalt = ptrs->remotesalt + MASTER_SALT_LEN;
  }
}

SSL_VERIFY_CB(dtls_trivial_verify_callback) {
  // TODO: add actuall verify routines here, if needed.
  (void)preverify_ok;
  (void)ctx;
  return 1;
}

SSL_CTX* dtls_ctx_init(int verify_mode, ssl_verify_cb* cb, const tlscfg* cfg) {
  SSL_CTX* ctx = SSL_CTX_new(DTLS_method());

  SSL_CTX_set_read_ahead(ctx, true);
  SSL_CTX_set_ecdh_auto(ctx, true);
  SSL_CTX_set_verify(ctx,
		     (verify_mode & DTLS_VERIFY_FINGERPRINT) ||
			     (verify_mode & DTLS_VERIFY_CERTIFICATE)
			 ? (SSL_VERIFY_PEER | SSL_VERIFY_FAIL_IF_NO_PEER_CERT)
			 : SSL_VERIFY_NONE,
		     !(verify_mode & DTLS_VERIFY_CERTIFICATE)
			 ? (cb ? cb : dtls_trivial_verify_callback)
			 : NULL);

  switch (cfg->profile) {
    case SRTP_PROFILE_AES128_CM_SHA1_80:
      SSL_CTX_set_tlsext_use_srtp(ctx, "SRTP_AES128_CM_SHA1_80");
      break;
    case SRTP_PROFILE_AES128_CM_SHA1_32:
      SSL_CTX_set_tlsext_use_srtp(ctx, "SRTP_AES128_CM_SHA1_32");
      break;
    default:
      SSL_CTX_free(ctx);
      return NULL;
  }

  if (!SSL_CTX_use_certificate(ctx, cfg->cert)) {
    SSL_CTX_free(ctx);
    return NULL;
  }

  if (!SSL_CTX_use_PrivateKey(ctx, cfg->pkey) ||
      !SSL_CTX_check_private_key(ctx)) {
    SSL_CTX_free(ctx);
    return NULL;
  }

  if (!SSL_CTX_set_cipher_list(ctx, cfg->cipherlist)) {
    SSL_CTX_free(ctx);
    return NULL;
  }

  return ctx;
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
  BIO* wbio = dtls_sess_get_wbio(sess);
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

  BIO* rbio = dtls_sess_get_rbio(sess);

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

ptrdiff_t dtls_do_handshake(dtls_sess* sess, const char* src, const char* dst) {
  if (sess->ssl == NULL ||
      (dtls_sess_get_state(sess) != DTLS_CONSTATE_ACT &&
       dtls_sess_get_state(sess) != DTLS_CONSTATE_ACTPASS)) {
    return -2;
  }
  if (dtls_sess_get_state(sess) == DTLS_CONSTATE_ACTPASS) {
    dtls_sess_set_state(sess, DTLS_CONSTATE_ACT);
  }
  SSL_do_handshake(sess->ssl);
  pthread_mutex_lock(&sess->lock);
  ptrdiff_t ret = dtls_sess_send_pending(sess, src, dst);
  pthread_mutex_unlock(&sess->lock);
  return ret;
}

srtp_key_material* srtp_get_key_material(dtls_sess* sess) {
  if (!SSL_is_init_finished(sess->ssl)) {
    return NULL;
  }

  srtp_key_material* km = calloc(1, sizeof(srtp_key_material));

  if (!SSL_export_keying_material(sess->ssl, km->material, sizeof(km->material),
				  "EXTRACTOR-dtls_srtp", 19, NULL, 0, 0)) {
    key_material_free(km);
    return NULL;
  }

  km->ispassive = sess->state;

  return km;
}

void key_material_free(srtp_key_material* km) {
  memset(km->material, 0, sizeof(km->material));
  free(km);
}

// function to print binary blobs as comma-separated hexadecimals.
int fprinthex(FILE* fp, const char* prefix, const void* b, size_t l) {
  int totallen = 0;
  const char* finger = (const char*)b;
  const char* end = finger + l;
  totallen += fprintf(fp, "%s:        %hhx", prefix, *(finger++));
  for (; finger != end; finger++) {
    totallen += fprintf(fp, ":%hhx", *finger);
  }
  totallen += fputs("\n\n", fp);
  return totallen;
}

// function to specifically print content srtp_key_ptrs objects.
int fprintkeymat(FILE* fp, const srtp_key_ptrs* ptrs) {
  return fputs("********\n", fp) +
	 fprinthex(fp, "localkey", ptrs->localkey, MASTER_KEY_LEN) +
	 fprinthex(fp, "remotekey", ptrs->remotekey, MASTER_KEY_LEN) +
	 fprinthex(fp, "localsalt", ptrs->localsalt, MASTER_SALT_LEN) +
	 fprinthex(fp, "remotesalt", ptrs->remotesalt, MASTER_SALT_LEN) +
	 fputs("********\n", fp);
}

// function to specifically print fingerprint of X509 objects.
int fprintfinger(FILE* fp, const char* prefix, const X509* cert) {
  unsigned char fingerprint[EVP_MAX_MD_SIZE];
  unsigned int size = sizeof(fingerprint);
  memset(fingerprint, 0, sizeof(fingerprint));
  if (!X509_digest(cert, EVP_sha512(), fingerprint, &size) || size == 0) {
    fprintf(stderr, "Failed to generated fingerprint from X509 object %p\n",
	    cert);
    return 0;
  }
  return fprinthex(fp, prefix, fingerprint, size);
}

tlscfg* dtls_build_tlscfg(void* cert_data, int cert_data_size, void* key_data,
			  int key_data_size) {
  tlscfg* cfg = (tlscfg*)calloc(1, sizeof(tlscfg));
  cfg->profile = SRTP_PROFILE_AES128_CM_SHA1_80;
  cfg->cipherlist = cipherlist;

  BIO* bio = BIO_new_mem_buf(cert_data, cert_data_size);
  if (NULL == (cfg->cert = PEM_read_bio_X509(bio, NULL, NULL, NULL))) {
    fputs("Fail to parse certificate file!\n", stderr);
    BIO_free(bio);
    return NULL;
  }
  fprintfinger(stdout, "Fingerprint of local cert is ", cfg->cert);
  BIO_free(bio);

  bio = BIO_new_mem_buf(key_data, key_data_size);
  if (NULL == (cfg->pkey = PEM_read_bio_PrivateKey(bio, NULL, NULL, NULL))) {
    fputs("Fail to parse private key file!\n", stderr);
    BIO_free(bio);
    return NULL;
  }
  BIO_free(bio);

  return cfg;
}

SSL_CTX* dtls_build_sslctx(tlscfg* cfg) {
  if (cfg == NULL) {
    return NULL;
  }
  return dtls_ctx_init(DTLS_VERIFY_FINGERPRINT, NULL, cfg);
}

dtls_sess* dtls_build_session(SSL_CTX* cfg, bool is_server) {
  return dtls_sess_new(cfg, is_server);
}

bool openssl_global_init() {
  OpenSSL_add_ssl_algorithms();
  SSL_load_error_strings();
  return SSL_library_init();
}

void dtls_session_cleanup(tlscfg* cfg, SSL_CTX* ssl_ctx,
			  dtls_sess* dtls_session) {
  if (dtls_session) {
    dtls_sess_free(dtls_session);
  }
  if (ssl_ctx) {
    SSL_CTX_free(ssl_ctx);
  }
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

void dtls_handle_incoming(dtls_sess* sess, const char* src, const char* dst,
			  void* buf, int len) {
  ptrdiff_t stat = dtls_sess_put_packet(sess, src, dst, buf, len);
  if (SSL_get_error(sess->ssl, stat) == SSL_ERROR_SSL) {
    fprintf(stderr,
	    "DTLS failure occurred on dtls session %p due to reason '%s'\n",
	    sess, ERR_reason_error_string(ERR_get_error()));
    return;
  }

  if (sess->type == DTLS_CONTYPE_EXISTING) {
    X509* peercert = SSL_get_peer_certificate(sess->ssl);
    if (peercert == NULL) {
      fprintf(stderr,
	      "No certificate was provided by the peer on dtls session %p\n",
	      sess);
      return;
    }
    fprintfinger(stdout, "Fingerprint of peer's cert is ", peercert);
    X509_free(peercert);

    srtp_key_material* km = srtp_get_key_material(sess);
    if (km == NULL) {
      fprintf(stderr,
	      "Unable to extract SRTP keying material from dtls session %p\n",
	      sess);
      return;
    }
    srtp_key_ptrs ptrs = {0, 0, 0, 0};
    srtp_key_material_extract(km, &ptrs);
    fprintkeymat(stdout, &ptrs);
    key_material_free(km);

    /* if we are a server
      if(sess->ssl == NULL || !SSL_is_init_finished(sess->ssl)){
	return;
      }

      SSL_clear(sess->ssl);
      if (sess->state == DTLS_CONSTATE_PASS) {
	SSL_set_accept_state(sess->ssl);
      } else {
	SSL_set_connect_state(sess->ssl);
      }
      sess->type = DTLS_CONTYPE_NEW;
    */
  }
}
