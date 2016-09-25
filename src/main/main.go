package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"

	"github.com/dhowden/tag"
	"github.com/dwbuiten/go-mediainfo/mediainfo"
	"github.com/eknkc/amber"
	"github.com/fhs/gompd/mpd"
	"github.com/jinzhu/gorm"
    _ "github.com/jinzhu/gorm/dialects/sqlite"
)

type Media struct {
	gorm.Model
	OriginalName string `gorm:"not null"`
	FileName string `gorm:"not null;primary_key"`
	AV int `gorm:"not null"`
}

type PlaylistMedia struct {
    PlaylistID int `gorm:"not null;index:p_s"`
    MediaID int `gorm:"not null;index:p_s"`
}

type Playlist struct {
  gorm.Model
  Name string `gorm:"not null"`
  Media []Media `gorm:"many2many:playlist_media;"`
}

var conn *mpd.Client

var db *gorm.DB

var templates, err = amber.CompileDir("views/", amber.DefaultDirOptions, amber.DefaultOptions)

var rootDir *string

const DEBUG = true

var currentPlaylist Playlist

func display(w http.ResponseWriter, tmpl string, data interface{}) {
	templates[tmpl].Execute(w, data)
}

func isAudioOrVideo(path string) (_ string) {
	info, err := mediainfo.Open(path)
	if err != nil {
		log.Fatalln(err)
		return ""
	}
	defer info.Close()

	// Find a video stream. If one is found, we are dealing with a video.
	_, err = info.Get("Format", 0, mediainfo.Video)
	if err != nil {
		// not a video!
		_, err = info.Get("Format", 0, mediainfo.Audio)
		if err != nil {
			// not an audio either!
			return ""
		}
		return "audio"
	}

	return "video"
}

func ComputeMd5(filePath string) (string, error) {
	var result []byte
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(result)), nil
}

func GetQueue() []mpd.Attrs {
	files, err := conn.PlaylistInfo(-1, -1)
	if err != nil {
		log.Println(err)
		return nil
	} else {
		return files
	}
}

func ClearQueue() {
	conn.Clear()
	currentPlaylist = Playlist{}
}

func AddToQueue(uri string) {
	conn.Add(uri)
}

func removeFromQueue(start int, end int) bool {
	err := conn.Delete(start, end)
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}

func Repeat(level int) {
	// 0 is mpd single false, and repeat false
	// 1 is mpd single false, and repeat true
	// 2 is mpd single true, and repeat true
	switch level {
	case 0:
		conn.Single(false)
		conn.Repeat(false)
	case 1:
		conn.Single(false)
		conn.Repeat(true)
	case 2:
		conn.Single(true)
		conn.Repeat(true)
	}
}

func GetCurrentSong() (mpd.Attrs){
	attrs, _ := conn.CurrentSong()
	return attrs
}

func GetAllMedia(aOrV int) (media []Media) {
	if aOrV < 0 {
		db.Find(&media)
	} else {
		db.Where("AV = ?", aOrV).Find(&media)
	}
	return
}

func GetPlaylists() (playlists []Playlist) {
	db.Model(&playlists)
	return
}

func GetPlayListAndMedia(playlistId int) (playlist Playlist, media []Media) {
	db.First(&playlist, playlistId).Related(&media)
	return
}

func GetPlayListMedia(playlistId int) (media []Media) {
	db.Joins("left join playlist_media pm on pm.media_id = media.id").Where(&PlaylistMedia{PlaylistID: playlistId}).Scan(&media)
	return
}

func LoadPlaylist(playlistId int) ([]Media) {
	playlist, media := GetPlayListAndMedia(playlistId)
	ClearQueue()
	for _, m := range media {
		AddToQueue(m.FileName)
	}
	currentPlaylist = playlist
	return media
}

func PlayMediaFromPlaylist(mediaId int, playlistId int) {
	var media []Media
	if int(currentPlaylist.ID) != playlistId {
		media = LoadPlaylist(playlistId)
	} else {
		media = currentPlaylist.Media
	}
	pos := 0
	for index, m := range media {
		if int(m.ID) == mediaId {
			pos = index
			break
		}
	}
	conn.Play(pos)
}

func ToggleRandom() (value bool) {
	status := GetStatus()
	random, _ := strconv.Atoi(status["random"])
	value = false
	if 1-random == 0 {
		value = false
	}
	value = true
	conn.Random(value)
	return
}

func StopPlayer() {
	conn.Stop()
}

func Next() {
	err := conn.Next()
	log.Println(err);
}

func Previous() {
	conn.Previous()
}

func StartPlayer(pos int) {
	conn.Play(pos)
}

func TogglePause() {
	status := GetStatus()
	if status["state"] == "play" {
		conn.Pause(true)
	} else if status["state"] == "stop" {
		StartPlayer(-1)
	} else {
		conn.Pause(false)
	}
}

func GetStatus() map[string]string {
	status, err := conn.Status()
	if err != nil {
		log.Println(err)
	}
	return status
}

func SetVolume(vol int) bool {
	// vol, err := strconv.Atoi(volume)
	// if err != nil {
	// 	return false
	// }
	if vol >= 0 && vol <= 100 {
		err := conn.SetVolume(vol)
		if err != nil {
			log.Print(err)
			return false
		}
		return true
	}
	return false
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		display(w, "player/music", nil)
	case "POST":
		err := r.ParseMultipartForm(100000)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		m := r.MultipartForm

		files := m.File["files"]
		for i, _ := range files {
			file, err := files[i].Open()
			defer file.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			//create destination file making sure the path is writeable.
			path := "/tmp/" + files[i].Filename
			dst, err := os.Create(path)
			defer dst.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if _, err := io.Copy(dst, file); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			av := isAudioOrVideo(path)
			if av == "audio" || av == "video" {
				// is this file already present? check that with checksum!
				checksum, err := ComputeMd5(path)
				newpath := *rootDir + av + "/" + checksum
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				os.Rename(path, newpath)

				if av == "audio" {
					cmdArgs := []string{"waveform.sh"}
					if _, err = exec.Command("sh", cmdArgs...).Output(); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
					conn.Update(newpath)

					db.Create(Media{OriginalName: files[i].Filename, FileName: newpath, AV: 0})
				} else {
					db.Create(Media{OriginalName: files[i].Filename, FileName: newpath, AV: 1})
				}
			}
			// remove the tmp
			os.Remove(path)
		}
		display(w, "player/music", "Upload successful.")
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func queueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeJs(GetQueue()))
	} else {
		w.WriteHeader(404)
	}
}

func coverArtHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		f, err := os.Open(*rootDir + "audio/" + r.FormValue("file"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		m, err := tag.ReadFrom(f)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", m.Picture().MIMEType)
		w.Header().Set("Content-Length", strconv.Itoa(len(m.Picture().Data)))
		if _, err := w.Write(m.Picture().Data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		w.WriteHeader(404)
	}
}

func playerStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		w.Write(makeJs(GetStatus()))
	} else {
		w.WriteHeader(404)
	}
}

func volumeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// if SetVolume(r.FormValue("volume")) {
		// 	w.WriteHeader(200)
		// } else {
		// 	http.Error(w, err.Error(), http.StatusInternalServerError)
		// }
	} else {
		w.WriteHeader(404)
	}
}

func makeJs(f interface{}) []byte {
	js, err := json.Marshal(f)
	if err != nil {
		log.Fatalln(err)
		return nil
	}
	return js
}

func UpdateUi() (map[string]interface{}) {
	status := GetStatus()
	data := make(map[string]interface{}, 10)
	data["time"] = status["time"]
	data["state"] = status["state"]
	repeat, _ := strconv.ParseInt(status["repeat"]+status["single"], 2, 64)
	data["repeat"] = repeat
	random, _ := strconv.Atoi(status["random"])
	data["random"] = random
	v, _ := strconv.Atoi(status["volume"])
	data["volume"] = v
	if currentPlaylist.ID != 0 {
		data["playlist"] = currentPlaylist
	}
	data["queue"] = GetQueue()
	data["playlists"] = GetPlaylists()
	data["currentsong"] = GetCurrentSong()

	// log.Println(data)
	return data
}

type SocketData struct {
	Action string
	Data interface{}
}

func main() {
	var err error

	var mpdHost = flag.String("h", "192.168.3.10", "Host for MPD")
	var mpdPort = flag.String("p", "6600", "Host for MPD")

	var webPort = flag.String("w", ":3000", "Web ip and port to listen on. or just :<port>")

	rootDir = flag.String("d", "/Users/amit/content/", "Host for MPD")

	conn, err = mpd.Dial("tcp", *mpdHost+":"+*mpdPort)
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	err = os.MkdirAll(*rootDir+"audio", os.ModePerm)
	err = os.MkdirAll(*rootDir+"video", os.ModePerm)
	err = os.MkdirAll(*rootDir+"meta/audio", os.ModePerm)
	err = os.MkdirAll(*rootDir+"meta/video", os.ModePerm)

	// if DEBUG {
	// 	os.Remove("./metadata.db")
	// }

	db, err = gorm.Open("sqlite3", "metadata.db")
	if err != nil {
		panic("failed to connect database")
	}
	defer db.Close()

	// Migrate the schema
	db.AutoMigrate(&Media{})
	db.AutoMigrate(Playlist{})
	db.AutoMigrate(PlaylistMedia{})

	mediainfo.Init()
	
	hub := newHub()
	go hub.run()
	
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	http.Handle("/", http.FileServer(http.Dir("./assets")))

	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/queue", queueHandler)
	http.HandleFunc("/coverArt", coverArtHandler)
	http.HandleFunc("/status", playerStatusHandler)
	http.HandleFunc("/volume", volumeHandler)

	http.ListenAndServe(*webPort, nil)
}
