#ifndef SRTP_H
#define SRTP_H

#include <string.h>
#include <stdlib.h>
#include <stdio.h>

#include "srtp2/srtp.h"

typedef struct rtp_packet {
	void *data;
	int len;
} rtp_packet;

srtp_t *srtp_create_session(void *client_write_key, void *server_write_key, char *profile);
rtp_packet *srtp_decrypt_packet(srtp_t * sess, void *data, int len);

#endif
