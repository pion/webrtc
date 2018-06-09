#include "srtp.h"

srtp_t *srtp_create_session(void *client_write_key, void *server_write_key, char *profile) {
  srtp_t *session = calloc(1, sizeof(srtp_t));
  srtp_policy_t policy;

  memset(&policy, 0x0, sizeof(srtp_policy_t));

  if (strcmp("SRTP_AES128_CM_SHA1_32", profile) == 0) {
    srtp_crypto_policy_set_aes_cm_128_hmac_sha1_32(&policy.rtp);
    srtp_crypto_policy_set_aes_cm_128_hmac_sha1_80(&policy.rtcp);
  } else if (strcmp("SRTP_AES128_CM_SHA1_80", profile) == 0) {
    srtp_crypto_policy_set_aes_cm_128_hmac_sha1_80(&policy.rtp);
    srtp_crypto_policy_set_aes_cm_128_hmac_sha1_80(&policy.rtcp);
  } else {
    return NULL;
  }

  policy.ssrc.value = 0;
  policy.next = NULL;

  policy.ssrc.type = ssrc_any_outbound;
  policy.key = server_write_key;
  if (srtp_create(session, &policy) != srtp_err_status_ok) {
    goto error;
  }

  policy.ssrc.type = ssrc_any_inbound;
  policy.key = client_write_key;
  if (srtp_create(session, &policy) != srtp_err_status_ok) {
    goto error;
  }

  return session;

error:
  free(session);
  return NULL;
}

rtp_packet *srtp_decrypt_packet(srtp_t *sess, void *data, int len) {
  rtp_packet *p = calloc(1, sizeof(rtp_packet));
  p->data = data;
  p->len = len;

  srtp_err_status_t status;
  if ((status = srtp_unprotect(*sess, p->data, &p->len)) != 0) {
    fprintf(stderr, "srtp_unprotect failed %d %d \n", status, len);
    goto error;
  }
  return p;

error:
  free(p);
  return NULL;
}
