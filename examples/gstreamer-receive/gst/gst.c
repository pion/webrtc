#include "gst.h"

#include <gst/app/gstappsrc.h>

static gboolean gstreamer_recieve_bus_call(GstBus *bus, GstMessage *msg, gpointer data) {
  GMainLoop *loop = (GMainLoop *)data;

  switch (GST_MESSAGE_TYPE(msg)) {

  case GST_MESSAGE_EOS:
    g_print("End of stream\n");
    g_main_loop_quit(loop);
    break;

  case GST_MESSAGE_ERROR: {
    gchar *debug;
    GError *error;

    gst_message_parse_error(msg, &error, &debug);
    g_free(debug);

    g_printerr("Error: %s\n", error->message);
    g_error_free(error);

    g_main_loop_quit(loop);
    break;
  }
  default:
    break;
  }

  return TRUE;
}

GstElement *gstreamer_recieve_create_pipeline() {
  gst_init(NULL, NULL);
  GError *error = NULL;
#define PIPELINE                                                                                                       \
  "appsrc format=time is-live=true do-timestamp=true name=src ! application/x-rtp, "                                   \
  "encoding-name=(string)VP8-DRAFT-IETF-01 "                                                                           \
  "! queue ! rtpvp8depay ! vp8dec ! videoconvert ! autovideosink"
  return gst_parse_launch(PIPELINE, &error);
}

void gstreamer_recieve_start_pipeline(GstElement *pipeline) {
  GMainLoop *loop;
  GstElement *source, *demuxer, *decoder, *conv, *sink;
  GstBus *bus;
  guint bus_watch_id;

  loop = g_main_loop_new(NULL, FALSE);

  bus = gst_pipeline_get_bus(GST_PIPELINE(pipeline));
  bus_watch_id = gst_bus_add_watch(bus, gstreamer_recieve_bus_call, loop);
  gst_object_unref(bus);

  gst_element_set_state(pipeline, GST_STATE_PLAYING);

  g_main_loop_run(loop);

  gst_element_set_state(pipeline, GST_STATE_NULL);

  gst_object_unref(GST_OBJECT(pipeline));
  g_source_remove(bus_watch_id);
  g_main_loop_unref(loop);
}

void gstreamer_recieve_stop_pipeline(GstElement *pipeline) { gst_element_set_state(pipeline, GST_STATE_NULL); }

void gstreamer_recieve_push_buffer(GstElement *pipeline, void *buffer, int len) {
  GstElement *src = gst_bin_get_by_name(GST_BIN(pipeline), "src");
  if (src != NULL) {
    gpointer p = g_memdup(buffer, len);
    GstBuffer *buffer = gst_buffer_new_wrapped(p, len);
    gst_app_src_push_buffer(GST_APP_SRC(src), buffer);
  }
}
