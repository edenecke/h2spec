package http2

import (
	"fmt"

	"golang.org/x/net/http2"

	"github.com/summerwind/h2spec/config"
	"github.com/summerwind/h2spec/spec"
)

func TheFlowControlWindow() *spec.TestGroup {
	tg := NewTestGroup("6.9.1", "The Flow-Control Window")

	// The sender MUST NOT send a flow-controlled frame with a length
	// that exceeds the space available in either of the flow-control
	// windows advertised by the receiver.
	tg.AddTestCase(&spec.TestCase{
		Desc:        "Sends SETTINGS frame to set the initial window size to 1 and sends HEADERS frame",
		Requirement: "The endpoint MUST NOT send a flow-controlled frame with a length that exceeds the space available.",
		Run: func(c *config.Config, conn *spec.Conn) error {
			var streamID uint32 = 1
			var actual spec.Event

			err := conn.Handshake()
			if err != nil {
				return err
			}

			headers := spec.CommonHeaders(c)
			hp1 := http2.HeadersFrameParam{
				StreamID:      streamID,
				EndStream:     true,
				EndHeaders:    true,
				BlockFragment: conn.EncodeHeaders(headers),
			}
			conn.WriteHeaders(hp1)

			// Get the length of response body.
			resLen := -1
			for resLen == -1 {
				ev := conn.WaitEvent()

				switch event := ev.(type) {
				case spec.EventDataFrame:
					resLen = int(event.Header().Length)
				}
			}

			// Skip this test case when the length of response body is 0.
			if resLen < 1 {
				return spec.ErrSkipped
			}

			settings := []http2.Setting{
				http2.Setting{
					ID:  http2.SettingInitialWindowSize,
					Val: 1,
				},
			}
			conn.WriteSettings(settings...)

			err = spec.VerifyFrameType(conn, http2.FrameSettings)
			if err != nil {
				return err
			}

			streamID += 2
			hp2 := http2.HeadersFrameParam{
				StreamID:      streamID,
				EndStream:     true,
				EndHeaders:    true,
				BlockFragment: conn.EncodeHeaders(headers),
			}
			conn.WriteHeaders(hp2)

			passed := false
			for !conn.Closed {
				ev := conn.WaitEvent()

				switch event := ev.(type) {
				case spec.EventDataFrame:
					actual = event
					passed = (event.Header().Length == 1)
				case spec.EventTimeout:
					if actual == nil {
						actual = event
					}
				default:
					actual = ev
				}

				if passed {
					break
				}
			}

			if !passed {
				expected := []string{
					fmt.Sprintf("DATA Frame (length:1, flags:0x00, stream_id:%d)", streamID),
				}

				return &spec.TestError{
					Expected: expected,
					Actual:   actual.String(),
				}
			}

			return nil
		},
	})

	// A sender MUST NOT allow a flow-control window to exceed 2^31-1
	// octets. If a sender receives a WINDOW_UPDATE that causes a
	// flow-control window to exceed this maximum, it MUST terminate
	// either the stream or the connection, as appropriate.
	// For streams, the sender sends a RST_STREAM with an error code
	// of FLOW_CONTROL_ERROR; for the connection, a GOAWAY frame with
	// an error code of FLOW_CONTROL_ERROR is sent.
	tg.AddTestCase(&spec.TestCase{
		Desc:        "Sends multiple WINDOW_UPDATE frames increasing the flow control window to above 2^31-1",
		Requirement: "The endpoint MUST sends a GOAWAY frame with a FLOW_CONTROL_ERROR code.",
		Run: func(c *config.Config, conn *spec.Conn) error {
			var actual spec.Event

			err := conn.Handshake()
			if err != nil {
				return err
			}

			conn.WriteWindowUpdate(0, 2147483647)
			conn.WriteWindowUpdate(0, 2147483647)

			passed := false
			for !conn.Closed {
				ev := conn.WaitEvent()

				switch event := ev.(type) {
				case spec.EventGoAwayFrame:
					actual = event
					passed = (event.ErrCode == http2.ErrCodeFlowControl)
				case spec.EventTimeout:
					if actual == nil {
						actual = event
					}
				default:
					actual = ev
				}

				if passed {
					break
				}
			}

			if !passed {
				expected := []string{
					fmt.Sprintf("GOAWAY Frame (Error Code: %s)", http2.ErrCodeFlowControl),
				}

				return &spec.TestError{
					Expected: expected,
					Actual:   actual.String(),
				}
			}

			return nil
		},
	})

	// A sender MUST NOT allow a flow-control window to exceed 2^31-1
	// octets. If a sender receives a WINDOW_UPDATE that causes a
	// flow-control window to exceed this maximum, it MUST terminate
	// either the stream or the connection, as appropriate.
	// For streams, the sender sends a RST_STREAM with an error code
	// of FLOW_CONTROL_ERROR; for the connection, a GOAWAY frame with
	// an error code of FLOW_CONTROL_ERROR is sent.
	tg.AddTestCase(&spec.TestCase{
		Desc:        "Sends multiple WINDOW_UPDATE frames increasing the flow control window to above 2^31-1 on a stream",
		Requirement: "The endpoint MUST sends a RST_STREAM frame with a FLOW_CONTROL_ERROR code.",
		Run: func(c *config.Config, conn *spec.Conn) error {
			var streamID uint32 = 1
			var actual spec.Event

			err := conn.Handshake()
			if err != nil {
				return err
			}

			headers := spec.CommonHeaders(c)
			hp := http2.HeadersFrameParam{
				StreamID:      streamID,
				EndStream:     false,
				EndHeaders:    true,
				BlockFragment: conn.EncodeHeaders(headers),
			}
			conn.WriteHeaders(hp)

			conn.WriteWindowUpdate(streamID, 2147483647)
			conn.WriteWindowUpdate(streamID, 2147483647)

			passed := false
			for !conn.Closed {
				ev := conn.WaitEvent()

				switch event := ev.(type) {
				case spec.EventRSTStreamFrame:
					actual = event
					if event.Header().StreamID == streamID {
						passed = (event.ErrCode == http2.ErrCodeFlowControl)
					}
				case spec.EventTimeout:
					if actual == nil {
						actual = event
					}
				default:
					actual = ev
				}

				if passed {
					break
				}
			}

			if !passed {
				expected := []string{
					fmt.Sprintf("RST_STREAM Frame (Error Code: %s)", http2.ErrCodeFlowControl),
				}

				return &spec.TestError{
					Expected: expected,
					Actual:   actual.String(),
				}
			}

			return nil
		},
	})

	return tg
}