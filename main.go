package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	booking := newBooking()
	// Pure-Go init (settings + embedded YAML/CSV) before the window is created so
	// saved geometry can seed the window options and avoid a resize flash.
	if err := booking.init(); err != nil {
		log.Fatalln("booking init:", err)
	}

	app := application.New(application.Options{
		Name:        "Kutyaguru Booking",
		Description: "Booked4us → Számlázz.hu CSV converter",
		Services: []application.Service{
			application.NewService(booking),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	s := booking.settings // loadSettings() guarantees non-zero W/H (1280/800)
	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Kutyaguru — Booked4us → Számlázz.hu",
		Width:            s.WindowW,
		Height:           s.WindowH,
		BackgroundColour: application.NewRGB(255, 255, 255),
		URL:              "/",
	})
	// First run (no saved position) keeps the default centered placement.
	if s.WindowX != 0 || s.WindowY != 0 {
		win.SetPosition(s.WindowX, s.WindowY)
	}
	booking.app = app
	booking.win = win

	// Persist geometry. Move/resize keep the in-memory geometry fresh (no disk
	// write per tick); WindowClosing flushes it. macOS Cmd+Q may skip
	// WindowClosing, so ServiceShutdown flushes the already-fresh values as a
	// backstop (see booking.go).
	win.OnWindowEvent(events.Common.WindowDidMove, func(e *application.WindowEvent) {
		booking.updateGeometry()
	})
	win.OnWindowEvent(events.Common.WindowDidResize, func(e *application.WindowEvent) {
		booking.updateGeometry()
	})
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		booking.updateGeometry()
		booking.saveSettings()
	})

	if err := app.Run(); err != nil {
		log.Fatalln(err)
	}
}
