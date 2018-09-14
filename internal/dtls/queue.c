#include "queue.h"

static inline record_st *new_record(queue_st *queue) {
  record_st *tmp;

  if (queue->cache != NULL) {
    tmp = queue->cache;
    queue->cache = tmp->next;
    queue->cache_size--;
  } else {
    tmp = (record_st *)malloc(sizeof *tmp);
  }

  return tmp;
}

static inline void del_record(queue_st *queue, record_st *node) {
  if (queue->cache_size > (queue->length / 8 + CACHE_SIZE)) {
    free(node);
  } else {
    node->msg.data = NULL;
    node->next = queue->cache;
    queue->cache = node;
    queue->cache_size++;
  }

  if (queue->cache_size > (queue->length / 4 + CACHE_SIZE * 10)) {
    record_st *tmp = queue->cache;
    queue->cache = tmp->next;
    free(tmp);
    queue->cache_size--;
  }
}

queue_st *queue_init() {
  int ret = 0;
  queue_st *queue = (queue_st *)malloc(sizeof(queue_st));
  memset(queue, 0, sizeof(queue_st));
  ret = pthread_cond_init(&queue->cond, NULL);
  if (ret != 0) {
    free(queue);
    return NULL;
  }

  ret = pthread_mutex_init(&queue->mutex, NULL);
  if (ret != 0) {
    pthread_cond_destroy(&queue->cond);
    free(queue);
    return NULL;
  }

  return queue;
}

int queue_put(queue_st *queue, void *data) {
  record_st *rec;
  pthread_mutex_lock(&queue->mutex);
  rec = new_record(queue);
  if (rec == NULL) {
    pthread_mutex_unlock(&queue->mutex);
    return ENOMEM;
  }
  rec->msg.data = data;

  rec->next = NULL;
  if (queue->last == NULL) {
    queue->last = rec;
    queue->first = rec;
  } else {
    queue->last->next = rec;
    queue->last = rec;
  }

  if (queue->length == 0) {
    pthread_cond_broadcast(&queue->cond);
  }

  queue->length++;
  pthread_mutex_unlock(&queue->mutex);

  return 0;
}

message_st *queue_get(queue_st *queue, const timespec_st *timeout) {
  if (queue == NULL) {
    return NULL;
  }

  timespec_st abstimeout;
  if (timeout) {
    timeval_st now;
    gettimeofday(&now, NULL);
    abstimeout.tv_sec = now.tv_sec + timeout->tv_sec;
    abstimeout.tv_nsec = (now.tv_usec * 1000) + timeout->tv_nsec;
    if (abstimeout.tv_nsec >= 1000000000) {
      abstimeout.tv_sec++;
      abstimeout.tv_nsec -= 1000000000;
    }
  }

  int ret = 0;
  pthread_mutex_lock(&queue->mutex);
  while (queue->first == NULL && ret != ETIMEDOUT) {
    if (timeout) {
      ret = pthread_cond_timedwait(&queue->cond, &queue->mutex, &abstimeout);
    } else {
      pthread_cond_wait(&queue->cond, &queue->mutex);
    }
  }

  if (ret == ETIMEDOUT) {
    pthread_mutex_unlock(&queue->mutex);
    return NULL;
  }

  record_st *front;
  front = queue->first;
  queue->first = queue->first->next;
  queue->length--;

  if (queue->first == NULL) {
    queue->last = NULL;
    queue->length = 0;
  }

  message_st *msg = (message_st *)malloc(sizeof(message_st));
  msg->data = front->msg.data;

  del_record(queue, front);
  pthread_mutex_unlock(&queue->mutex);

  return 0;
}

long queue_size(queue_st *queue) {
  long counter;
  pthread_mutex_lock(&queue->mutex);
  counter = queue->length;
  pthread_mutex_unlock(&queue->mutex);
  return counter;
}

int queue_destroy(queue_st *queue) {
  record_st *rec;
  record_st *next;
  record_st *recs[2];
  int ret, i;
  if (queue == NULL) {
    return EINVAL;
  }

  pthread_mutex_lock(&queue->mutex);
  recs[0] = queue->first;
  recs[1] = queue->cache;
  for (i = 0; i < 2; i++) {
    rec = recs[i];
    while (rec) {
      next = rec->next;
      free(rec->msg.data);
      free(rec);
      rec = next;
    }
  }

  pthread_mutex_unlock(&queue->mutex);
  ret = pthread_mutex_destroy(&queue->mutex);
  pthread_cond_destroy(&queue->cond);

  return ret;
}