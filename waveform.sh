#!/bin/bash

# script to update waveforms of any files that have no waveform generated yet!

LIST=`sqlite3 metadata.db "SELECT checksum, file FROM media"`;

convert -size 660X60 canvas:#4d5d6e background.png

# For each row
for ROW in $LIST; do
	# Parsing data (sqlite3 returns a pipe separated string)
	checksum=`expr "$ROW" : '^\([^\|]*\)'`
	fileName=`expr "$ROW" : '^[^\|]*\|\(.*\)$'`

	DIR=`dirname "${fileName}"`

	if [ ! -f "$DIR/../meta/audio/$checksum.png" ]; then
		ffmpeg -i "$fileName" -filter_complex "aformat=channel_layouts=mono,showwavespic=s=660x60,format=rgba,colorkey=black,colorchannelmixer=rr=0.0:gg=0.0:bb=0.0" -frames:v 1 "$DIR/../meta/audio/$checksum.png"
		convert background.png "$DIR/../meta/audio/$checksum.png" -gravity center -compose CopyOpacity -composite -channel A -negate "$DIR/../meta/audio/$checksum.png"
	fi
done

rm background.png