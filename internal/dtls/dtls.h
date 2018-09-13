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

typedef struct dtls_cert_st {
  X509 *cert;
  EVP_PKEY *pkey;
} dtls_cert_st;

typedef struct dtls_sess_st {
  SSL *ssl;

  enum dtls_con_state state;
  enum dtls_con_type type;
} dtls_sess_st;

typedef struct dtls_decrypted {
  void *buf;
  int len;
} dtls_decrypted;

#define PROFILE_STRING_LENGTH 23
#define SRTP_MASTER_KEY_KEY_LEN 16
#define SRTP_MASTER_KEY_SALT_LEN 14

typedef struct dtls_cert_pair {
  char client_write_key[SRTP_MASTER_KEY_KEY_LEN + SRTP_MASTER_KEY_SALT_LEN];
  char server_write_key[SRTP_MASTER_KEY_KEY_LEN + SRTP_MASTER_KEY_SALT_LEN];
  char profile[PROFILE_STRING_LENGTH];
  int key_length;
} dtls_cert_pair;

void dtls_init();

dtls_cert_st *dtls_build_certificate();

SSL_CTX *dtls_build_ssl_context(dtls_cert_st *cert);

dtls_sess_st *dtls_build_session(SSL_CTX *ctx, bool is_offer);

ptrdiff_t dtls_do_handshake(dtls_sess_st *sess, char *local, char *remote);

dtls_decrypted *dtls_handle_incoming(dtls_sess_st *sess, void *buf, int len);

bool dtls_handle_outgoing(dtls_sess_st *sess, void *buf, int len);

dtls_cert_pair *dtls_get_certpair(dtls_sess_st *sess);

char *dtls_certificate_fingerprint(dtls_cert_st *cfg);

void dtls_session_cleanup(SSL_CTX *ctx, dtls_sess_st *sess, dtls_cert_st *cert);

#endif
