package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App manages the Wails lifecycle and owns the Booking instance.
type App struct {
	ctx     context.Context
	booking *Booking
}

func NewApp() (*App, *Booking) {
	b := newBooking()
	return &App{booking: b}, b
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.booking.ctx = ctx
	if err := a.booking.init(); err != nil {
		panic("booking init: " + err.Error())
	}
}

// domReady restores the saved window size and position.
func (a *App) domReady(ctx context.Context) {
	s := a.booking.settings
	if s.WindowW > 0 && s.WindowH > 0 {
		runtime.WindowSetSize(ctx, s.WindowW, s.WindowH)
	}
	if s.WindowX != 0 || s.WindowY != 0 {
		runtime.WindowSetPosition(ctx, s.WindowX, s.WindowY)
	}
}

// beforeClose saves the current window geometry before the app exits.
func (a *App) beforeClose(ctx context.Context) bool {
	x, y := runtime.WindowGetPosition(ctx)
	w, h := runtime.WindowGetSize(ctx)
	a.booking.settings.WindowX = x
	a.booking.settings.WindowY = y
	a.booking.settings.WindowW = w
	a.booking.settings.WindowH = h
	a.booking.saveSettings()
	return false // allow close
}
