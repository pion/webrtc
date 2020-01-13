#!/bin/bash
set -e

fps=25
width=640
height=360
bitrate=2M
length=00:01:00 # hour:minute:second

ffmpeg -f lavfi -i testsrc \
    -y `# force overwrite output file` \
    -t ${length} `# length` \
    -vf "\
        scale=w=${width}: h=${height}: force_original_aspect_ratio=decrease, `# resolution` \
        pad=${width}:${height}:(ow-iw)/2:(oh-ih)/2, `# black bars` \
        fps=${fps}, \
        format=yuv420p \
    " \
    -sn \
    -c:v libx264 \
    -r ${fps} `# frame per second` \
    -x264opts "keyint=${fps}:min-keyint=${fps}:no-scenecut" `# group of pictures length` \
    -x264-params "nal-hrd=cbr" -b:v ${bitrate} -minrate ${bitrate} -maxrate ${bitrate} -bufsize ${bitrate} `# constant bitrate` \
    -bsf:v h264_metadata=delete_filler=1 \
    -profile:v baseline -level 4.0 \
    output.h264