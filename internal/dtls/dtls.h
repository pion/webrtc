#ifndef DTLS_FOO_H
#define DTLS_FOO_H

#include <openssl/bio.h>
#include <openssl/ssl.h>
#include <openssl/err.h>

#include <string.h>

#include <assert.h>
#include <stdbool.h>
#include <stdint.h>

#include <pthread.h>

#define MASTER_KEY_LEN 16
#define MASTER_SALT_LEN 14

enum dtls_verify_mode {
  DTLS_VERIFY_NONE = 0,               /*!< Don't verify anything */
  DTLS_VERIFY_FINGERPRINT = (1 << 0), /*!< Verify the fingerprint */
  DTLS_VERIFY_CERTIFICATE = (1 << 1), /*!< Verify the certificate */
};

enum dtls_con_state {
  DTLS_CONSTATE_ACT, //Endpoint is willing to inititate connections.
  DTLS_CONSTATE_PASS, //Endpoint is willing to accept connections.
  DTLS_CONSTATE_ACTPASS, //Endpoint is willing to both accept and initiate connections
  DTLS_CONSTATE_HOLDCONN, //Endpoint does not want the connection to be established right now
};

enum dtls_con_type {
  DTLS_CONTYPE_NEW = false, //Endpoint wants to use a new connection
  DTLS_CONTYPE_EXISTING = true, //Endpoint wishes to use existing connection
};

enum srtp_profile {
  SRTP_PROFILE_RESERVED=0,
  SRTP_PROFILE_AES128_CM_SHA1_80=1,
  SRTP_PROFILE_AES128_CM_SHA1_32=2,
};

#define SSL_VERIFY_CB(x) int (x)(int preverify_ok, X509_STORE_CTX *ctx)
typedef SSL_VERIFY_CB(ssl_verify_cb);
extern SSL_VERIFY_CB(dtls_trivial_verify_callback);

typedef struct tlscfg {
  X509* cert;
  EVP_PKEY* pkey;
  enum srtp_profile profile;
  const char* cipherlist;
} tlscfg;

typedef struct dtls_sess {
  SSL* ssl;
  enum dtls_con_state state;
  enum dtls_con_type type;
  pthread_mutex_t lock;
} dtls_sess;

typedef struct srtp_key_material{
  uint8_t material[(MASTER_KEY_LEN + MASTER_SALT_LEN) * 2];
  enum dtls_con_state ispassive;
}srtp_key_material;

typedef struct srtp_key_ptrs {
  const uint8_t* localkey;
  const uint8_t* remotekey;
  const uint8_t* localsalt;
  const uint8_t* remotesalt;
} srtp_key_ptrs;

bool openssl_global_init();

tlscfg *dtls_build_tlscfg(void *cert_data, int cert_data_size, void *key_data, int key_data_size);
SSL_CTX *dtls_build_sslctx(tlscfg *cfg);
dtls_sess* dtls_build_session(SSL_CTX* cfg, bool is_server);

ptrdiff_t dtls_do_handshake(dtls_sess* sess, const char *src, const char *dst);
void dtls_handle_incoming(dtls_sess* sess, const char *src, const char *dst, void *buf, int len);

void dtls_session_cleanup(tlscfg *cfg, SSL_CTX *ssl_ctx, dtls_sess *dtls_session);


#endif
