//go:build linux && !386

// Copyright 2013-2015 go-diameter authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sm

import (
	"testing"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
)

func requireSCTP(t *testing.T) {
	t.Helper()
	ln, err := diam.MultistreamListen("sctp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("SCTP not available: %v", err)
	}
	ln.Close()
}

func TestHandleCER_HandshakeMetadataSCTP(t *testing.T) {
	requireSCTP(t)
	testHandleCER_HandshakeMetadata(t, "sctp")
}

func testClient_Handshake_CustomIP_SCTP(t *testing.T) {
	requireSCTP(t)
	testClient_Handshake_CustomIP(t, "sctp")
}

// TestStateMachineSCTP establishes a connection with a test SCTP server and
// sends a Re-Auth-Request message to ensure the handshake was
// completed and that the RAR handler has context from the peer.
func TestStateMachineSCTP(t *testing.T) {
	requireSCTP(t)
	// SCTP sockets are multi-homed: on a host with a non-loopback address
	// (e.g. CI runners) getLocalAddresses() prefers the real address over
	// loopback, so the auto-detected Host-IP-Address would not match the
	// loopback value testStateMachine's expected CEA asserts. Pin it to
	// loopback for this run (a local copy, so the shared serverSettings and
	// the TCP test are untouched).
	settings := *serverSettings
	settings.HostIPAddresses = []datatype.Address{localhostAddress}
	testStateMachine(t, "sctp", &settings)
}
