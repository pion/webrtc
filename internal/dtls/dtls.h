#ifndef DTLS_H
#define DTLS_H

#include <openssl/bio.h>
#include <openssl/err.h>
#include <openssl/ssl.h>

#include <pthread.h>

#include <stdbool.h>
#include <string.h>

#define SRTP_MASTER_KEY_KEY_LEN 16
#define SRTP_MASTER_KEY_SALT_LEN 14

enum dtls_con_state {
  DTLS_CONSTATE_ACT,      // Endpoint is willing to inititate connections.
  DTLS_CONSTATE_PASS,     // Endpoint is willing to accept connections.
  DTLS_CONSTATE_ACTPASS,  // Endpoint is willing to both accept and initiate connections
  DTLS_CONSTATE_HOLDCONN, // Endpoint does not want the connection to be established right now
};

enum dtls_con_type {
  DTLS_CONTYPE_NEW = false,     // Endpoint wants to use a new connection
  DTLS_CONTYPE_EXISTING = true, // Endpoint wishes to use existing connection
};

typedef struct tlscfg {
  X509 *cert;
  EVP_PKEY *pkey;
} tlscfg;

typedef struct dtls_sess {
  SSL *ssl;
  enum dtls_con_state state;
  enum dtls_con_type type;
  pthread_mutex_t lock;
} dtls_sess;

#define PROFILE_STRING_LENGTH 23
#define SRTP_MASTER_KEY_KEY_LEN 16
#define SRTP_MASTER_KEY_SALT_LEN 14

typedef struct dtls_handle_incoming_return {
  char client_write_key[SRTP_MASTER_KEY_KEY_LEN + SRTP_MASTER_KEY_SALT_LEN];
  char server_write_key[SRTP_MASTER_KEY_KEY_LEN + SRTP_MASTER_KEY_SALT_LEN];
  char profile[PROFILE_STRING_LENGTH];
  int key_length;
} dtls_handle_incoming_return;

bool openssl_global_init();

tlscfg *dtls_build_tlscfg();
SSL_CTX *dtls_build_sslctx(tlscfg *cfg);
dtls_sess *dtls_build_session(SSL_CTX *cfg, bool is_server);

ptrdiff_t dtls_do_handshake(dtls_sess *sess, const char *src, const char *dst);
dtls_handle_incoming_return *dtls_handle_incoming(dtls_sess *sess, const char *src, const char *dst, void *buf,
                                                  int len);
char *dtls_tlscfg_fingerprint(tlscfg *cfg);

void dtls_session_cleanup(SSL_CTX *ssl_ctx, dtls_sess *dtls_session);
void dtls_tlscfg_cleanup(tlscfg *cfg);

#endif
