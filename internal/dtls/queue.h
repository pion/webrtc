#ifndef QUEUE_H
#define QUEUE_H

#include <errno.h>
#include <pthread.h>
#include <stdlib.h>
#include <string.h>
#include <sys/time.h>

#define CACHE_SIZE 256

typedef struct timespec timespec_st;
typedef struct timeval timeval_st;

typedef struct message_st {
  void *data;
} message_st;

typedef struct record_st record_st;
struct record_st {
  message_st msg;
  record_st *next;
};

typedef struct queue_st {
  long length;
  pthread_mutex_t mutex;
  pthread_cond_t cond;
  record_st *first, *last;
  record_st *cache;
  long cache_size;
} queue_st;

queue_st *queue_init();

int queue_put(queue_st *queue, void *data);

message_st *queue_get(queue_st *queue, const timespec_st *timeout);

long queue_size(queue_st *queue);

int queue_destroy(queue_st *queue);

#endif