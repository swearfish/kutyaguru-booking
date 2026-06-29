package main

import "context"

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
		// Assets are embedded — this should never fail in production.
		panic("booking init: " + err.Error())
	}
}
