package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joebjoe/me/internal/env"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

var config = new(struct {
	FileID   string `env:"FILE_ID,required"`
	Username string `env:"AUTH_USER,required"`
	Password string `env:"AUTH_PASSWORD,required,base64"`
})

func main() {
	env.MustLoad(config)

	e := echo.New()
	e.HideBanner = true
	go startLogLevelListener(e)

	h := newHandler(config.FileID)

	e.GET("/", h.GET)
	e.PUT("/:file_id", h.PUT, withBasicAuth(config.Username, config.Password))

	go e.Start(":80")

	//graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	e.Logger.Print("waiting for quit")
	<-quit
	e.Logger.Print("quit received")
}

// HANDLER

type handler struct {
	fileID string
}

func newHandler(fileID string) *handler {
	return &handler{fileID: fileID}
}

// GET

const driveURLFmt = "https://drive.google.com/file/d/%s/view"

func (h *handler) GET(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	return c.Redirect(http.StatusPermanentRedirect, fmt.Sprintf(driveURLFmt, h.fileID))
}

// PUT

type putRequest struct {
	Current string `param:"file_id"`
	New     string `json:"new_file_id"`
}

func (h *handler) PUT(c echo.Context) error {
	req := new(putRequest)
	if err := c.Bind(req); err != nil {
		return err
	}

	if req.Current != h.fileID || req.New == "" {
		return echo.NewHTTPError(http.StatusBadRequest)
	}

	if err := os.Setenv("FILE_ID", req.New); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to set value at FILE_ID: %v", err))
	}

	h.fileID = req.New

	return c.String(http.StatusOK, fmt.Sprintf(driveURLFmt, h.fileID))
}

// LOGGING

var logLevelDict = map[string]log.Lvl{
	"DEBUG": log.DEBUG,
	"INFO":  log.INFO,
	"WARN":  log.WARN,
	"ERROR": log.ERROR,
	"OFF":   log.OFF,
}

func startLogLevelListener(e *echo.Echo) {
	for {
		wait := time.After(time.Minute * 15)
		lvl, ok := os.LookupEnv("LOG_LEVEL")
		if !ok || lvl == "" {
			e.Logger.Printf("LOG_LEVEL is not set; defaulting to DEBUG")
			e.Logger.SetLevel(log.DEBUG)
			<-wait
			continue
		}
		e.Logger.SetLevel(logLevelDict[lvl])
		<-wait
	}
}

// BASIC AUTH

func withBasicAuth(user, pass string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			u, p, ok := c.Request().BasicAuth()
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized)
			}
			if u != user || p != pass {
				return echo.NewHTTPError(http.StatusUnauthorized)
			}
			return next(c)
		}
	}
}
