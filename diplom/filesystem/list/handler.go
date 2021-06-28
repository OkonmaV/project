package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"strings"
	"thin-peak/logs/logger"

	"github.com/big-larry/suckhttp"
	"github.com/big-larry/suckutils"
)

type Handler struct {
	Path string
	tmpl *template.Template
}

type fileInfo struct {
	Name     string
	RealSize int
	Size     string
	IsDir    bool
	ModTime  string
}

func NewHandler(path string) (*Handler, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("Path not set")
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, errors.New("Path is not dir")
	}

	templData, err := ioutil.ReadFile("index.html")
	if err != nil {
		return nil, err
	}

	templ, err := template.New("index").Parse(string(templData))
	if err != nil {
		return nil, err
	}
	return &Handler{path, templ}, nil
}

func (handler *Handler) Handle(req *suckhttp.Request, l *logger.Logger) (*suckhttp.Response, error) {
	filename := suckutils.ConcatTwo(handler.Path, req.Uri.Path)

	stat, err := os.Stat(filename)
	if err != nil {
		l.Error("Stat", err)
		return suckhttp.NewResponse(404, "Not found"), nil
	}
	if !stat.IsDir() {
		l.Error("Stat", errors.New("Path is not dir"))
		return suckhttp.NewResponse(404, "Not found"), nil
	}

	files, err := os.ReadDir(filename)
	if err != nil {
		l.Error("ReadDir", err)
		return suckhttp.NewResponse(404, "Not found"), nil
	}

	result := make([]fileInfo, 0, len(files))
	for _, f := range files {
		i, err := f.Info()
		if err != nil {
			l.Warning("Listing", err.Error())
			continue
		}
		result = append(result, fileInfo{
			IsDir:    i.IsDir(),
			Name:     i.Name(),
			RealSize: int(i.Size()),
			Size:     ByteCountSI(i.Size()),
			ModTime:  i.ModTime().Local().Format("02/01/2006 15:04:05"),
		})
	}

	buf := &bytes.Buffer{}
	if err = handler.tmpl.Execute(buf, struct {
		Files []fileInfo
		Dir   string
	}{
		Files: result,
		Dir:   req.Uri.Path,
	}); err != nil {
		l.Error("ExecuteHtml", err)
		return suckhttp.NewResponse(500, "Internal Server Error"), nil
	}

	response := suckhttp.NewResponse(200, "OK")
	response.AddHeader(suckhttp.Content_Type, "text/html; charset=utf-8")
	response.SetBody(buf.Bytes())
	return response, nil
}

func ByteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

func ByteCountIEC(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
