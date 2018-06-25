#ifndef GST_H
#define GST_H

#include <glib.h>
#include <gst/gst.h>
#include <stdint.h>
#include <stdlib.h>

GstElement *gstreamer_recieve_create_pipeline();
void gstreamer_recieve_start_pipeline(GstElement *pipeline);
void gstreamer_recieve_stop_pipeline(GstElement *pipeline);
void gstreamer_recieve_push_buffer(GstElement *pipeline, void *buffer, int len);

#endif
