package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/icza/session"
)

var rootPath string
var users *Users

type MessagePage struct {
	Header  string
	Message interface{}
}

type ViewPage struct {
	Header  string
	Up      string
	Options interface{}
	DirInfo string
	MPre    interface{}
	MPost   interface{}
	Dirs    []interface{}
	Medias  []interface{}
	Others  []interface{}
	Path    string
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		t, _ := template.ParseFiles("templates/login.gtpl")
		t.Execute(w, nil)
	} else {
		r.ParseForm()
		// logic part of log in
		userName := r.FormValue("username")
		err := users.Login(userName, r.FormValue("password"))
		if err != nil {
			t, _ := template.ParseFiles("templates/message.gtpl")
			msg := MessagePage{
				Header: "Login Failed",
				Message: template.HTML(
					"<p>Incorrect name and/or password was provided</p>" +
						"<p><a href=\"./login\">Retry</a></p>",
				),
			}
			t.Execute(w, msg)
			s := session.Get(r)
			if s != nil {
				// logout of existing session if login failed
				session.Remove(s, w)
			}
		} else {
			s := session.NewSessionOptions(&session.SessOptions{
				CAttrs: map[string]interface{}{"UserName": userName},
			})
			session.Add(s, w)
			http.Redirect(w, r, "./view?path=.", 301)
		}
	}
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	s := session.Get(r)
	if s == nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Logout Failed",
			Message: template.HTML(
				"<p>You were not logged in</p>" +
					"<p><a href=\"./login\">Login</a></p>",
			),
		}
		t.Execute(w, msg)
	} else {
		session.Remove(s, w)
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Logged Out",
			Message: template.HTML(
				"<p><a href=\"./login\">Login</a></p>",
			),
		}
		t.Execute(w, msg)
	}
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "./view?path=.", 301)
}

func ViewHandler(w http.ResponseWriter, r *http.Request) {
	s := session.Get(r)
	if s == nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Not Logged In",
			Message: template.HTML(
				"<p>You were not logged in</p>" +
					"<p><a href=\"./login\">Login</a></p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	userReqPath := r.URL.Query().Get("path")
	scaling := r.URL.Query().Get("scaling")
	showVid := r.URL.Query().Get("showvid")
	height := r.URL.Query().Get("height")
	if scaling == "" {
		scaling = "FillHorizontal"
	}
	if userReqPath == "" {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "No Path Specified",
			Message: template.HTML(
				"<p>You must specify path to resource with 'path='</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	if showVid == "" {
		showVid = "No"
	}
	if height == "" {
		height = "25"
	}
	reqPath := rootPath + "/" + strings.TrimPrefix(userReqPath, "/")
	// escape userReqPath for subequent links
	userReqPathDisplay := userReqPath
	userReqSegments := strings.Split(userReqPath, "/")
	userReqPath = ""
	for i, userReqSegment := range userReqSegments {
		if i != 0 {
			userReqPath += "/"
		}
		userReqPath += url.QueryEscape(userReqSegment)
	}
	fmt.Printf("ViewHandler: %s -> %s\n", r.URL.RequestURI(), reqPath)
	f, err := os.Stat(reqPath)
	if os.IsNotExist(err) {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Path not Found",
			Message: template.HTML(
				"<p>Can not find '" + reqPath + "'</p>",
			),
		}
		t.Execute(w, msg)
		return
	} else if err != nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "File status check failed",
			Message: template.HTML(
				"<p>Failed to check status for  '" + reqPath + "'</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	if strings.HasSuffix(strings.ToLower(reqPath), ".jpg") ||
		strings.HasSuffix(strings.ToLower(reqPath), ".jpeg") {
		img, err := os.Open(reqPath)
		if err != nil {
			fmt.Println("'" + reqPath + "' failed to open: " + err.Error())
			return // no response
		}
		defer img.Close()
		w.Header().Set("Content-Type", "image/jpeg")
		io.Copy(w, img)
	} else if strings.HasSuffix(strings.ToLower(reqPath), ".png") {
		img, err := os.Open(reqPath)
		if err != nil {
			fmt.Println("'" + reqPath + "' failed to open: " + err.Error())
			return // no response
		}
		defer img.Close()
		w.Header().Set("Content-Type", "image/png")
		io.Copy(w, img)
	} else if strings.HasSuffix(strings.ToLower(reqPath), ".gif") {
		img, err := os.Open(reqPath)
		if err != nil {
			fmt.Println("'" + reqPath + "' failed to open: " + err.Error())
			return // no response
		}
		defer img.Close()
		w.Header().Set("Content-Type", "image/gif")
		io.Copy(w, img)
	} else if strings.HasSuffix(strings.ToLower(reqPath), ".webm") ||
		strings.HasSuffix(strings.ToLower(reqPath), ".mkv") ||
		strings.HasSuffix(strings.ToLower(reqPath), ".mp4") {
		video, err := os.Open(reqPath)
		if err != nil {
			fmt.Println("'" + reqPath + "' failed to open: " + err.Error())
			return
		}
		defer video.Close()
		http.ServeContent(w, r, reqPath, time.Now(), video)
	} else if strings.HasSuffix(strings.ToLower(reqPath), ".txt") {
		img, err := os.Open(reqPath)
		if err != nil {
			fmt.Println("'" + reqPath + "' failed to open: " + err.Error())
			return // no response
		}
		defer img.Close()
		w.Header().Set("Content-Type", "text/plain")
		io.Copy(w, img)
	} else if strings.HasSuffix(strings.ToLower(reqPath), ".html") {
		img, err := os.Open(reqPath)
		if err != nil {
			fmt.Println("'" + reqPath + "' failed to open: " + err.Error())
			return // no response
		}
		defer img.Close()
		w.Header().Set("Content-Type", "text/html")
		io.Copy(w, img)
	} else if f.IsDir() {
		files, _ := ioutil.ReadDir(reqPath)
		cur := "./view?path=" + userReqPath
		curVid := "showvid=" + showVid
		curScaling := "scaling=" + scaling
		curHeight := "height=" + height
		options := "<span style=\"margin-right: 10px\">"
		if showVid == "Yes" {
			options = options + "<a href=\"" + cur + "&showvid=No&" +
				curScaling + "&" + curHeight + "\">-Vid</a> "
		} else {
			options = options + "<a href=\"" + cur + "&showvid=Yes&" +
				curScaling + "&" + curHeight + "\">+Vid</a> "
		}
		options = options + "</span><span style=\"margin-right: 10px\">"
		if scaling != "FillHorizontal" {
			options = options + "<a href=\"" + cur + "&scaling=FillHorizontal&" +
				curVid + "&" + curHeight + "\">H</a> "
		} else {
			options = options + "H "
		}
		options = options + "</span><span style=\"margin-right: 10px\">"
		if scaling != "FillVertical" {
			options = options + "<a href=\"" + cur + "&scaling=FillVertical&" +
				curVid + "&" + curHeight + "\">V</a> "
		} else {
			options = options + "V "
		}
		options = options + "</span><span style=\"margin-right: 5px\">"
		if scaling != "Thumbnail" {
			options = options + "<a href=\"" + cur + "&scaling=Thumbnail&" +
				curVid + "&" + curHeight + "\">T</a>:"
		} else {
			options = options + "T:"
		}
		options = options + "</span><span style=\"margin-right: 5px\">"
		if height != "25" {
			options = options + "<a href=\"" + cur + "&" + curScaling +
				"&" + curVid + "&height=25\">&#188;</a>"
		} else {
			options = options + "&#188;"
		}
		options = options + "</span><span style=\"margin-right: 10px\">"
		if height != "50" {
			options = options + "<a href=\"" + cur + "&" + curScaling +
				"&" + curVid + "&height=50\">&#189;</a> "
		} else {
			options = options + "&#189; "
		}
		options = options + "</span><span style=\"margin-right: 5px\">"
		if scaling != "List" {
			options = options + "<a href=\"" + cur + "&scaling=List&" +
				curVid + "&" + curHeight + "\">L</a> "
		} else {
			options = options + "L "
		}
		options = options + "</span>"
		options = options + "</span><span style=\"margin-right: 10px\">"
		if scaling != "ListPreview" {
			options = options + "<a href=\"" + cur + "&scaling=ListPreview&" +
				curVid + "&" + curHeight + "\">P</a> "
		} else {
			options = options + "P "
		}
		options = options + "</span><span style=\"margin-right: 10px\">"
		options = options + "<a href=\"thumbgen?path=" +
			url.QueryEscape(userReqPath) + "&done=" + url.QueryEscape(userReqPath) +
			"&" + curVid + "&" + curScaling + "&" + curHeight +
			"\">" +
			"TG </a>"
		options = options + "</span>"
		page := ViewPage{
			Header: userReqPathDisplay,
			Up: "./view?path=" + filepath.Dir("./"+userReqPath) +
				"&" + curVid + "&" + curScaling + "&" + curHeight,
			Options: template.HTML(options),
			MPre:    "",
			MPost:   "",
			Dirs:    make([]interface{}, 0),
			Medias:  make([]interface{}, 0),
			Others:  make([]interface{}, 0),
			Path:    userReqPath,
		}
		fileCount := 0
		unknownCount := 0
		for _, file := range files {
			fileNameEscaped := url.QueryEscape(file.Name())
			if file.IsDir() {
				page.Dirs = append(page.Dirs,
					template.HTML("<p>&#128193; <a href=\"./view?"+
						"path="+userReqPath+"/"+fileNameEscaped+
						"&"+curVid+
						"&"+curScaling+
						"&"+curHeight+
						"\">"+
						file.Name()+"</a></p>"))
			} else if scaling == "List" { //&& isListable(file.Name()) {
				page.Others = append(page.Others,
					template.HTML("<p><a href=\""+
						cur+"/"+fileNameEscaped+"\">"+
						file.Name()+"</a></p>"))
				fileCount++
			} else if scaling == "ListPreview" && isListable(file.Name()) {
				page.MPre = template.HTML("<table>")
				page.MPost = template.HTML("</table>")
				if isImage(file.Name()) {
					prefix := "<tr><td style=\"vertical-align: middle;\"><a href=\"" + cur + "/" +
						fileNameEscaped + "\">"
					suffix := "</a></td><td><a href=\"" + cur + "/" +
						fileNameEscaped + "\"><span style=\"vertical-align: middle;\">" + file.Name() + "</span></a></td></tr>"
					imgAttr := "height=\"100px\" style=\"vertical-align: middle;\""
					page.Medias = append(page.Medias,
						template.HTML(prefix+"<img src=\"./view?path="+
							userReqPath+"/"+fileNameEscaped+"\" "+
							imgAttr+"> "+suffix))
				} else if isVideo(file.Name()) {
					prefix := "<tr><td style=\"vertical-align: middle;\"><a href=\"" + cur + "/" +
						fileNameEscaped + "\">"
					suffix := "</a></td><td><a href=\"" + cur + "/" +
						fileNameEscaped + "\"><span style=\"vertical-align: middle;\">" + file.Name() + "</span></a></td></tr>"
					imgAttr := "height=\"100px\" style=\"vertical-align: middle;\""
					page.Medias = append(page.Medias,
						template.HTML(prefix+"<img src=\"./view?path="+
							userReqPath+"/"+fileNameEscaped+".thumb.jpg\" "+
							imgAttr+"> "+suffix))
				} else {
					page.Others = append(page.Others,
						template.HTML("<p><a href=\""+
							cur+"/"+fileNameEscaped+"\">"+
							file.Name()+"</a></p>"))
					fileCount++
				}
				fileCount++
			} else {
				imgAttr := ""
				switch scaling {
				case "FillHorizontal":
					imgAttr = "width=\"100%\""
				case "FillVertical":
					imgAttr = "height=\"100%\""
				case "Thumbnail":
					imgAttr = "height=\"" + height + "%\""
				}
				if isImage(file.Name()) &&
					!strings.HasSuffix(file.Name(), ".thumb.jpg") {
					prefix := ""
					suffix := ""
					if scaling == "Thumbnail" {
						prefix = "<a href=\"" + cur + "/" +
							fileNameEscaped + "\">"
						suffix = "</a>"
					}
					page.Medias = append(page.Medias,
						template.HTML(prefix+"<img src=\"./view?path="+
							userReqPath+"/"+fileNameEscaped+"\" "+
							imgAttr+">"+suffix+" "))
					fileCount++
				} else if isVideo(file.Name()) {
					prefix := ""
					suffix := ""
					video := ""
					if showVid != "Yes" {
						prefix = "<a href=\"" + cur + "/" +
							fileNameEscaped + "\">"
						video = "<img " + imgAttr +
							" src=\"" + cur + "/" +
							fileNameEscaped + ".thumb.jpg\">"
						suffix = "</a>"
					} else {
						video = "<video " + imgAttr +
							" src=\"" + cur + "/" +
							fileNameEscaped + "\" autoplay loop muted></video>"
					}
					page.Medias = append(page.Medias,
						template.HTML(prefix+video+suffix))
					fileCount++
				} else if isListable(file.Name()) {
					page.Others = append(page.Others,
						template.HTML("<p><a href=\""+
							cur+"/"+fileNameEscaped+"\">"+
							file.Name()+"</a></p>"))
					fileCount++
				} else if !strings.HasSuffix(file.Name(), ".thumb.jpg") {
					unknownCount++
				}
			}
		}
		page.DirInfo = fmt.Sprintf("%s: %d%s%d%s%d%s", userReqPathDisplay,
			len(page.Dirs), " dirs, ", fileCount, " media files and ",
			unknownCount, " unknown files")
		t, _ := template.ParseFiles("templates/view.gtpl")
		t.Execute(w, page)
	} else {
		// prompt download otherwise
		img, err := os.Open(reqPath)
		if err != nil {
			fmt.Println("'" + reqPath + "' failed to open: " + err.Error())
			return // no response
		}
		defer img.Close()
		fi, err := img.Stat()
		if err != nil {
			// Could not obtain stat, handle error
		}
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(img.Name())+"\"")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
		io.Copy(w, img)
		/*
			t, _ := template.ParseFiles("templates/message.gtpl")
			msg := MessagePage{
					Header: "Unable to Handle File",
					Message: template.HTML(
						"<p>Unable to handle '" + reqPath + "'</p>",
					),
			}
			t.Execute(w, msg)
			return
		*/
	}
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid HTTP Method", 400)
		return
	}
	s := session.Get(r)
	if s == nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Not Logged In",
			Message: template.HTML(
				"<p>You were not logged in</p>" +
					"<p><a href=\"./login\">Login</a></p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	url := r.FormValue("url")
	filePath := filepath.Join(rootPath, r.FormValue("path"), path.Base(url))
	filePath = strings.Replace(filePath, ":", "_", -1)
	filePath = strings.Replace(filePath, "?", "_", -1)
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Get Failed",
			Message: template.HTML(
				"<p>Fetch failed</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	defer resp.Body.Close()
	mime := resp.Header.Get("Content-Type")
	if mime == "image/jpeg" && !strings.HasSuffix(filePath, ".jpg") {
		filePath = filePath + ".jpg"
	} else if mime == "image/png" && !strings.HasSuffix(filePath, ".png") {
		filePath = filePath + ".png"
	}
	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Get Failed",
			Message: template.HTML(
				"<p>File open failed</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	defer out.Close()

	// Write the body to file
	fmt.Println("mime: '" + mime + "' writing to " + filePath)
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Get Failed",
			Message: template.HTML(
				"<p>File write failed</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	http.Redirect(w, r, r.Header.Get("Referer"), 302)
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid HTTP Method", 400)
		return
	}
	s := session.Get(r)
	if s == nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Not Logged In",
			Message: template.HTML(
				"<p>You were not logged in</p>" +
					"<p><a href=\"./login\">Login</a></p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	fmt.Println("UploadHandler: parsing")
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Upload Failed",
			Message: template.HTML(
				"<p>Failed to parse multipart form</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	fmt.Println("UploadHandler: getting file info")
	file, header, err := r.FormFile("upload")
	if err != nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Upload Failed",
			Message: template.HTML(
				"<p>Failed to get file</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	defer file.Close()
	filePath := filepath.Join(rootPath, r.FormValue("path"), header.Filename)
	fmt.Println("UploadHandler: (" + strconv.FormatInt(header.Size, 10) + " bytes) writing to " + filePath)
	// fileContents, err := ioutil.ReadAll(file)
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0644)
	defer f.Close()
	if err != nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Upload Failed",
			Message: template.HTML(
				"<p>Failed to write file</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	io.Copy(f, file)
	http.Redirect(w, r, r.Header.Get("Referer"), 302)
}

func ThumbnailGenerator(w http.ResponseWriter, r *http.Request) {
	s := session.Get(r)
	if s == nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Not Logged In",
			Message: template.HTML(
				"<p>You were not logged in</p>" +
					"<p><a href=\"./login\">Login</a></p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	userReqPath, err := url.QueryUnescape(r.URL.Query().Get("path"))
	if err != nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Failed to unescape path",
			Message: template.HTML(
				"<p>Failed to check status for  '" + r.URL.Query().Get("path") + "'</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	done := r.URL.Query().Get("done")
	curVid := "showvid=" + r.URL.Query().Get("showvid")
	curScaling := "scaling=" + r.URL.Query().Get("scaling")
	curHeight := "height=" + r.URL.Query().Get("height")
	reqPath := rootPath + "/" + userReqPath
	f, err := os.Stat(reqPath)
	if os.IsNotExist(err) {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Path not Found",
			Message: template.HTML(
				"<p>Can not find '" + reqPath + "'</p>",
			),
		}
		t.Execute(w, msg)
		return
	} else if err != nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "File status check failed",
			Message: template.HTML(
				"<p>Failed to check status for  '" + reqPath + "'</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	if !f.IsDir() {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Path is not a directory",
			Message: template.HTML(
				"<p>'" + reqPath + "' needs to be a directory</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
	files, _ := ioutil.ReadDir(reqPath)
	output := ""
	cmd := "ffmpeg"
	for _, file := range files {
		fileName := reqPath + "/" + file.Name()
		if isVideo(fileName) {
			args := []string{"-y", "-ss", "00:10:00", "-i",
				fileName, "-vframes", "1", fileName + ".thumb.jpg"}
			out, err := exec.Command(cmd, args...).CombinedOutput()
			output = output + fmt.Sprintf("%s", out)
			if err != nil {
				output = output + err.Error()
			}
			output = output + "\n"
			_, err = os.Stat(fileName + ".thumb.jpg")
			if os.IsNotExist(err) {
				args := []string{"-y", "-ss", "00:00:01", "-i",
					fileName, "-vframes", "1", fileName + ".thumb.jpg"}
				out, err := exec.Command(cmd, args...).CombinedOutput()
				output = output + fmt.Sprintf("%s", out)
				if err != nil {
					output = output + err.Error()
				}
				output = output + "\n"
			}
		}
	}
	if done == "" {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Thumbnail Generation Output",
			Message: template.HTML(
				"<div align=\"left\"><pre>" + output + "</pre></div>",
			),
		}
		t.Execute(w, msg)
	} else {
		http.Redirect(w, r, "./view?path="+done+
			"&"+curVid+"&"+curScaling+"&"+curHeight, 301)
	}
}

func ShutdownHandler(w http.ResponseWriter, r *http.Request) {
	s := session.Get(r)
	if s == nil {
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "Not Logged In",
			Message: template.HTML(
				"<p>You were not logged in</p>" +
					"<p><a href=\"./login\">Login</a></p>",
			),
		}
		t.Execute(w, msg)
		return
	} else {
		cmd := exec.Command("/usr/bin/shutdown", "now")
		cmd.Run()
		t, _ := template.ParseFiles("templates/message.gtpl")
		msg := MessagePage{
			Header: "System Shut Down",
			Message: template.HTML(
				"<p>System is shutting down</p>",
			),
		}
		t.Execute(w, msg)
		return
	}
}

func isImage(name string) bool {
	lName := strings.ToLower(name)
	return strings.HasSuffix(lName, ".png") ||
		strings.HasSuffix(lName, ".gif") ||
		strings.HasSuffix(lName, ".jpg") ||
		strings.HasSuffix(lName, ".jpeg")
}

func isVideo(name string) bool {
	lName := strings.ToLower(name)
	return strings.HasSuffix(lName, ".mp4") ||
		strings.HasSuffix(lName, ".mkv") ||
		strings.HasSuffix(lName, ".webm")
}

func isText(name string) bool {
	lName := strings.ToLower(name)
	return strings.HasSuffix(lName, ".txt") ||
		strings.HasSuffix(lName, ".text") ||
		strings.HasSuffix(lName, ".html")
}

func isListable(name string) bool {
	lName := strings.ToLower(name)
	return (isImage(lName) || isVideo(lName) || isText(lName)) &&
		!strings.HasSuffix(lName, ".thumb.jpg")
}

func usage() {
	fmt.Println("usage: mediaserve [path] [users-file] [static-path] [listen host:port] (cert) (key)\n")
}

func main() {
	if len(os.Args) != 5 && len(os.Args) != 7 {
		usage()
		return
	}
	rootPath = os.Args[1]
	users = NewUsers()
	err := users.LoadFromFile(os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	session.Global.Close()
	session.Global = session.NewCookieManagerOptions(session.NewInMemStore(), &session.CookieMngrOptions{AllowHTTP: true})
	fs := http.FileServer(http.Dir(os.Args[3]))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/view", ViewHandler)
	http.HandleFunc("/thumbgen", ThumbnailGenerator)
	http.HandleFunc("/login", LoginHandler)
	http.HandleFunc("/logout", LogoutHandler)
	http.HandleFunc("/upload", UploadHandler)
	http.HandleFunc("/get", GetHandler)
	http.HandleFunc("/shutdown", ShutdownHandler)
	http.HandleFunc("/", RootHandler)
	if len(os.Args) == 6 {
		err = http.ListenAndServeTLS(os.Args[4], os.Args[5], os.Args[6], nil)
	} else {
		err = http.ListenAndServe(os.Args[4], nil)
	}
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
