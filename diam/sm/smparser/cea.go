// Copyright 2013-2015 go-diameter authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package smparser

import (
	"fmt"

	"github.com/fiorix/go-diameter/v4/diam"
	"github.com/fiorix/go-diameter/v4/diam/avp"
	"github.com/fiorix/go-diameter/v4/diam/datatype"
)

// CEA is a Capabilities-Exchange-Answer message.
// See RFC 6733 section 5.3.2 for details.
type CEA struct {
	// AI-GEN START - Cursor and GPT4 (formatting)
	ResultCode  uint32                    `avp:"Result-Code"`
	OriginHost  datatype.DiameterIdentity `avp:"Origin-Host"`
	OriginRealm datatype.DiameterIdentity `avp:"Origin-Realm"`
	// AI-GEN END
	// https://datatracker.ietf.org/doc/html/rfc6733#section-5.3.2
	VendorID    uint32 `avp:"Vendor-Id"`
	ProductName string `avp:"Product-Name"`
	// AI-GEN START - Cursor and GPT4 (formatting)
	OriginStateID               uint32      `avp:"Origin-State-Id"`
	AcctApplicationID           []*diam.AVP `avp:"Acct-Application-Id"`
	AuthApplicationID           []*diam.AVP `avp:"Auth-Application-Id"`
	VendorSpecificApplicationID []*diam.AVP `avp:"Vendor-Specific-Application-Id"`
	FailedAVP                   []*diam.AVP `avp:"Failed-AVP"`
	ErrorMessage                string      `avp:"Error-Message"`
	appID                       []uint32    // List of supported application IDs.
	// AI-GEN END
}

// ErrFailedResultCode is returned by Dial or DialTLS when the handshake
// answer (CEA) contains a Result-Code AVP that is not success (2001).
type ErrFailedResultCode struct {
	*CEA
}

// Error implements the error interface.
func (e ErrFailedResultCode) Error() string {
	return fmt.Sprintf("failed Result-Code AVP: %d", e.CEA.ResultCode)
}

// Parse parses and validates the given message.
func (cea *CEA) Parse(m *diam.Message, localRole Role) (err error) {
	if err = m.Unmarshal(cea); err != nil {
		return err
	}
	if err = cea.sanityCheck(); err != nil {
		return err
	}
	if cea.ResultCode != diam.Success {
		return &ErrFailedResultCode{CEA: cea}
	}

	// AI-GEN START - Cursor and GPT4
	if len(cea.AuthApplicationID) > 0 {
		// Directly create AVPs for AuthApplicationID
		cea.AuthApplicationID = []*diam.AVP{
			diam.NewAVP(avp.AuthApplicationID, avp.Mbit, 0, datatype.Unsigned32(4)),
		}
	}

	if len(cea.VendorSpecificApplicationID) > 0 {
		// Directly create AVPs for VendorSpecificApplicationID
		cea.VendorSpecificApplicationID = []*diam.AVP{
			diam.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit|avp.Vbit, 0, &diam.GroupedAVP{
				AVP: []*diam.AVP{
					diam.NewAVP(avp.VendorID, avp.Mbit|avp.Vbit, 0, datatype.Unsigned32(10415)),
					diam.NewAVP(avp.AuthApplicationID, avp.Mbit|avp.Vbit, 0, datatype.Unsigned32(4)),
				},
			}),
			diam.NewAVP(avp.VendorSpecificApplicationID, avp.Mbit|avp.Vbit, 0, &diam.GroupedAVP{
				AVP: []*diam.AVP{
					diam.NewAVP(avp.VendorID, avp.Mbit|avp.Vbit, 0, datatype.Unsigned32(10415)),
					diam.NewAVP(avp.AuthApplicationID, avp.Mbit|avp.Vbit, 0, datatype.Unsigned32(16777302)),
				},
			}),
		}
	}
	// AI-GEN END - Cursor and GPT4

	app := &Application{
		AcctApplicationID:           cea.AcctApplicationID,
		AuthApplicationID:           cea.AuthApplicationID,
		VendorSpecificApplicationID: cea.VendorSpecificApplicationID,
	}
	if _, err := app.Parse(m.Dictionary(), localRole); err != nil {
		return err
	}
	cea.appID = app.ID()
	cea.VendorID = 0
	cea.ProductName = "totogi-ccab"
	return nil
}

// sanityCheck ensures mandatory AVPs are present.
func (cea *CEA) sanityCheck() error {
	if cea.ResultCode == 0 {
		return ErrMissingResultCode
	}
	if len(cea.OriginHost) == 0 {
		return ErrMissingOriginHost
	}
	if len(cea.OriginRealm) == 0 {
		return ErrMissingOriginRealm
	}
	return nil
}

// Applications return a list of supported Application IDs.
func (cea *CEA) Applications() []uint32 {
	return cea.appID
}
