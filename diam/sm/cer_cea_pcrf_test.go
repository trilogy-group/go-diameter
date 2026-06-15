// Copyright 2013-2015 go-diameter authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package sm

import (
	"testing"
	"time"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
	"github.com/fiorix/go-diameter/v4/diam/diamtest"
	"github.com/fiorix/go-diameter/v4/diam/dict"
)

// pcrfServerSettings advertises a distinct PCRF identity so the CEA identity
// selection can be observed on the wire.
var pcrfServerSettings = &Settings{
	OriginHost:       "ocs.example.com",
	OriginRealm:      "ocs-realm.example.com",
	VendorID:         13,
	ProductName:      "go-diameter",
	OriginStateID:    datatype.Unsigned32(1),
	FirmwareRevision: 1,
	PcrfHost:         "pcrf.example.com",
	PcrfRealm:        "pcrf-realm.example.com",
}

// advertisedAppIDs collects every application id advertised in a CEA, both the
// top-level Auth/Acct-Application-Id AVPs and the ones nested inside grouped
// Vendor-Specific-Application-Id AVPs.
func advertisedAppIDs(t *testing.T, m *diam.Message) map[uint32]bool {
	t.Helper()
	ids := map[uint32]bool{}
	for _, a := range m.AVP {
		switch a.Code {
		case avp.AuthApplicationID, avp.AcctApplicationID:
			ids[uint32(a.Data.(datatype.Unsigned32))] = true
		case avp.VendorSpecificApplicationID:
			grouped, ok := a.Data.(*diam.GroupedAVP)
			if !ok {
				continue
			}
			for _, inner := range grouped.AVP {
				if inner.Code == avp.AuthApplicationID || inner.Code == avp.AcctApplicationID {
					ids[uint32(inner.Data.(datatype.Unsigned32))] = true
				}
			}
		}
	}
	return ids
}

// originIdentity returns the Origin-Host/Origin-Realm advertised in a CEA.
func originIdentity(t *testing.T, m *diam.Message) (string, string) {
	t.Helper()
	var host, realm string
	for _, a := range m.AVP {
		switch a.Code {
		case avp.OriginHost:
			host = string(a.Data.(datatype.DiameterIdentity))
		case avp.OriginRealm:
			realm = string(a.Data.(datatype.DiameterIdentity))
		}
	}
	return host, realm
}

// exchangeCER sends a CER advertising the given application ids (as top-level
// Auth-Application-Id AVPs) and returns the CEA the server answers with.
func exchangeCER(t *testing.T, settings *Settings, appIDs ...uint32) *diam.Message {
	t.Helper()
	sm := New(settings)
	srv := diamtest.NewServer(sm, dict.Default)
	defer srv.Close()

	mc := make(chan *diam.Message, 1)
	mux := diam.NewServeMux()
	mux.HandleFunc("CEA", func(c diam.Conn, m *diam.Message) {
		mc <- m
	})
	cli, err := diam.Dial(srv.Addr, mux, dict.Default)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	m := diam.NewRequest(diam.CapabilitiesExchange, 0, dict.Default)
	m.NewAVP(avp.OriginHost, avp.Mbit, 0, clientSettings.OriginHost)
	m.NewAVP(avp.OriginRealm, avp.Mbit, 0, clientSettings.OriginRealm)
	m.NewAVP(avp.HostIPAddress, avp.Mbit, 0, localhostAddress)
	m.NewAVP(avp.VendorID, avp.Mbit, 0, clientSettings.VendorID)
	m.NewAVP(avp.ProductName, 0, 0, clientSettings.ProductName)
	for _, id := range appIDs {
		m.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(id))
	}
	if _, err = m.WriteTo(cli); err != nil {
		t.Fatal(err)
	}

	select {
	case resp := <-mc:
		return resp
	case err := <-mux.ErrorReports():
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("No CEA received")
	}
	return nil
}

// TestCEA_AdvertisesOnlySupportedApps verifies that the CEA advertises only the
// four applications this adapter supports (Gy, Rx, Gx, Sy), even though the
// loaded dictionary contains more applications (NASREQ, Base Accounting, S6a,
// SWx).
func TestCEA_AdvertisesOnlySupportedApps(t *testing.T) {
	cea := exchangeCER(t, pcrfServerSettings, diam.GX_CHARGING_CONTROL_APP_ID)
	if !testResultCode(cea, diam.Success) {
		t.Fatalf("Unexpected result code.\n%s", cea)
	}

	got := advertisedAppIDs(t, cea)
	want := map[uint32]bool{
		diam.CHARGING_CONTROL_APP_ID:    true,
		diam.RX_APP_ID:                  true,
		diam.GX_CHARGING_CONTROL_APP_ID: true,
		diam.DIAMETER_SY_APP_ID:         true,
	}
	for id := range want {
		if !got[id] {
			t.Errorf("CEA missing supported application id %d", id)
		}
	}
	for id := range got {
		if !want[id] {
			t.Errorf("CEA advertises unsupported application id %d", id)
		}
	}
	if len(got) != len(want) {
		t.Errorf("CEA advertised %d apps, want %d (%v)", len(got), len(want), got)
	}
}

// TestCEA_PcrfIdentitySelection verifies the Origin-Host/Origin-Realm chosen in
// the CEA based on the applications requested in the CER.
func TestCEA_PcrfIdentitySelection(t *testing.T) {
	tests := []struct {
		name      string
		appIDs    []uint32
		wantHost  string
		wantRealm string
	}{
		{"gx only", []uint32{diam.GX_CHARGING_CONTROL_APP_ID}, "pcrf.example.com", "pcrf-realm.example.com"},
		{"rx only", []uint32{diam.RX_APP_ID}, "pcrf.example.com", "pcrf-realm.example.com"},
		{"gx and rx", []uint32{diam.GX_CHARGING_CONTROL_APP_ID, diam.RX_APP_ID}, "pcrf.example.com", "pcrf-realm.example.com"},
		{"gy only", []uint32{diam.CHARGING_CONTROL_APP_ID}, "ocs.example.com", "ocs-realm.example.com"},
		{"sy only", []uint32{diam.DIAMETER_SY_APP_ID}, "ocs.example.com", "ocs-realm.example.com"},
		{"gx and gy", []uint32{diam.GX_CHARGING_CONTROL_APP_ID, diam.CHARGING_CONTROL_APP_ID}, "ocs.example.com", "ocs-realm.example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cea := exchangeCER(t, pcrfServerSettings, tc.appIDs...)
			if !testResultCode(cea, diam.Success) {
				t.Fatalf("Unexpected result code.\n%s", cea)
			}
			host, realm := originIdentity(t, cea)
			if host != tc.wantHost {
				t.Errorf("Origin-Host = %q, want %q", host, tc.wantHost)
			}
			if realm != tc.wantRealm {
				t.Errorf("Origin-Realm = %q, want %q", realm, tc.wantRealm)
			}
		})
	}
}

// TestCEA_NoPcrfConfigUsesDefaultIdentity verifies that when PcrfHost/PcrfRealm
// are unset, a Gx CER still gets the default Origin-Host/Origin-Realm.
func TestCEA_NoPcrfConfigUsesDefaultIdentity(t *testing.T) {
	cea := exchangeCER(t, serverSettings, diam.GX_CHARGING_CONTROL_APP_ID)
	if !testResultCode(cea, diam.Success) {
		t.Fatalf("Unexpected result code.\n%s", cea)
	}
	host, realm := originIdentity(t, cea)
	if host != string(serverSettings.OriginHost) {
		t.Errorf("Origin-Host = %q, want %q", host, serverSettings.OriginHost)
	}
	if realm != string(serverSettings.OriginRealm) {
		t.Errorf("Origin-Realm = %q, want %q", realm, serverSettings.OriginRealm)
	}
}
