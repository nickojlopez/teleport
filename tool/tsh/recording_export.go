/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"

	"github.com/gravitational/trace"
	"github.com/icza/mjpeg"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
)

const (
	framesPerSecond  = 30
	frameDelayMillis = float64(1000) / framesPerSecond
)

func onExportRecording(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyClient, err := tc.ConnectToProxy(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}
	defer proxyClient.Close()

	authClient := proxyClient.CurrentCluster()
	defer authClient.Close()

	var screen *image.NRGBA
	var movie mjpeg.AviWriter

	count := 0
	lastEmitted := int64(-1)
	buf := new(bytes.Buffer)

	evts, errs := authClient.StreamSessionEvents(cf.Context, session.ID(cf.SessionID), 0)
loop:
	for {
		select {
		case err := <-errs:
			return trace.Wrap(err)
		case <-cf.Context.Done():
			return cf.Context.Err()
		case evt, more := <-evts:
			if !more {
				break loop
			}

			switch evt := evt.(type) {
			case *apievents.SessionStart:
				return trace.BadParameter("only desktop recordings can be exported")
			case *apievents.WindowsDesktopSessionEnd:
				break loop
			case *apievents.DesktopRecording:
				msg, err := tdp.Decode(evt.Message)
				if err != nil {
					log.Warnf("failed to decode desktop recording message: %v", err)
					break loop
				}

				switch msg := msg.(type) {
				case tdp.ClientScreenSpec:
					if screen != nil {
						return trace.BadParameter("invalid recording: received multiple screen specs")
					}
					// Use the dimensions in the ClientScreenSpec to allocate
					// our virtual canvas and video file.
					// Note: this works because we don't currently support resizing
					// the window during a session. If this changes, we'd have to
					// find the maximum window size first.
					log.Debugf("allocating %dx%d screen", msg.Width, msg.Height)
					screen = image.NewNRGBA(image.Rectangle{
						Min: image.Pt(0, 0),
						Max: image.Pt(int(msg.Width), int(msg.Height)),
					})
					fname := cf.IdentityFileOut
					if fname == "" {
						fname = cf.SessionID + ".avi"
					}
					movie, err = mjpeg.New(fname, int32(msg.Width), int32(msg.Height), framesPerSecond)
					if err != nil {
						return trace.Wrap(err)
					}
				case tdp.PNG2Frame:
					count++
					if screen == nil {
						return trace.BadParameter("this session is missing required start metadata")
					}

					fragment, err := png.Decode(bytes.NewReader(msg.Data()))
					if err != nil {
						return trace.WrapWithMessage(err, "couldn't decode PNG")
					}

					// draw the fragment from this message on the screen
					draw.Draw(
						screen,
						image.Rect(
							// add one to bottom and right dimenstion, as RDP
							// bounds are inclusive
							int(msg.Left()), int(msg.Top()),
							int(msg.Right()+1), int(msg.Bottom()+1),
						),
						fragment,
						fragment.Bounds().Min,
						draw.Src,
					)

					// emit a frame if there's been enough of a time lapse between last event
					delta := evt.DelayMilliseconds - lastEmitted
					framesToEmit := int64(float64(delta) / frameDelayMillis)
					log.Debugf("%dms since last frame, emitting %d frames", delta, framesToEmit)
					if framesToEmit > 0 {
						buf.Reset()
						if err := jpeg.Encode(buf, screen, nil); err != nil {
							return trace.Wrap(err)
						}
						for i := 0; i < int(framesToEmit); i++ {
							if err := movie.AddFrame(buf.Bytes()); err != nil {
								return trace.Wrap(err)
							}
						}
						lastEmitted = evt.DelayMilliseconds
					}

				}
			default:
				log.Debugf("got unexpected audit event %T", evt)
			}
		}
	}

	return trace.Wrap(movie.Close())
}
