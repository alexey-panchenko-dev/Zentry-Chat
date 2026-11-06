# Synthetic encoded audio fixtures

These files are generated silence, not microphone recordings or proof of audible browser playback.

Generator: FFmpeg `N-92722-gf22fcd4483` (2018-12 build) from npm package `@ffmpeg-installer/win32-x64@4.1.0`, downloaded only into a temporary work folder. Package tarball SHA-1 reported by npm: `17e8699b5798d4c60e36e2d6326a8ebe5e95a2c5`. No FFmpeg runtime dependency or system installation is required by the API/tests.

Generation commands (FFmpeg executable name varies by OS):

```sh
ffmpeg -f lavfi -i anullsrc=r=48000:cl=mono -t 0.2 -c:a libopus silence.webm
ffmpeg -f lavfi -i anullsrc=r=48000:cl=mono -t 0.2 -c:a libopus silence.ogg
ffmpeg -f lavfi -i anullsrc=r=48000:cl=mono -t 0.2 -c:a aac silence.mp4
ffmpeg -f lavfi -i anullsrc=r=48000:cl=mono -t 0.2 -c:a aac -movflags frag_keyframe+empty_moov+default_base_moof silence-fragmented.mp4
```

The normal and fragmented MP4 fixtures exercise both nonfragmented sample tables and empty initialization durations with fragment runs. Tests derive structurally equivalent WebM unknown-sized Segment/Cluster variants and reject missing tracks/blocks, damaged page checksums, box bounds, truncated files, and declared durations over 120 seconds.

Validation is structural and bounded. It checks Opus WebM/Ogg and AAC/Opus MP4 audio tracks and sample payloads, without fully decoding codecs. WebM lacing and Ogg codecs other than Opus are not currently supported. Recorder-supplied duration remains unverified metadata; known container timestamps/durations over 120 seconds are rejected.
