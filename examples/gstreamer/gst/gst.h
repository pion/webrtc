#ifndef GST_H
#define GST_H

#include <glib.h>
#include <gst/gst.h>
#include <stdint.h>

GstElement *gst_create_pipeline();
void gst_start_pipeline(GstElement *pipeline);
void gst_stop_pipeline(GstElement *pipeline);
void gst_push_buffer(GstElement *pipeline, void *buffer, int len);

#endif