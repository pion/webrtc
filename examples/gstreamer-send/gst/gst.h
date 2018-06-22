#ifndef GST_H
#define GST_H

#include <glib.h>
#include <gst/gst.h>
#include <stdint.h>
#include <stdlib.h>

extern void goHandlePipelineBuffer(void *buffer, int bufferLen);

GstElement *gst_create_pipeline();
void gst_start_pipeline(GstElement *pipeline);
void gst_stop_pipeline(GstElement *pipeline);

#endif
