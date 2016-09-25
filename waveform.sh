#!/bin/bash

# script to update waveforms of any files that have no waveform generated yet!

LIST=`sqlite3 metadata.db "SELECT FileName FROM media where AV = 0"`;

convert -size 660X60 canvas:#4d5d6e background.png

# For each row
for ROW in $LIST; do
	# Parsing data (sqlite3 returns a pipe separated string)
	fileName=`echo $ROW`

	DIR=`dirname "${fileName}"`

	if [ ! -f "$DIR/../meta/audio/$fileName.png" ]; then
		ffmpeg -i "$fileName" -filter_complex "aformat=channel_layouts=mono,showwavespic=s=660x60,format=rgba,colorkey=black,colorchannelmixer=rr=0.0:gg=0.0:bb=0.0" -frames:v 1 "$DIR/../meta/audio/$fileName.png"
		convert background.png "$DIR/../meta/audio/$fileName.png" -gravity center -compose CopyOpacity -composite -channel A -negate "$DIR/../meta/audio/$fileName.png"
	fi
done

rm background.png