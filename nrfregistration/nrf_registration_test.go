// SPDX-FileCopyrightText: 2025 Canonical Ltd.
//
// SPDX-License-Identifier: Apache-2.0
//
/*
 * NRF Registration Unit Testcases
 *
 */
package nrfregistration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/omec-project/ausf/consumer"
	"github.com/omec-project/openapi/models"
)

func TestHandleNewConfig_EmptyConfig_DeregisterNF_StopTimer(t *testing.T) {
	var isSendDeregisterNFInstanceCalled bool
	testCases := []struct {
		name                         string
		sendDeregisterNFInstanceMock func() (*models.ProblemDetails, error)
	}{
		{
			name: "Success",
			sendDeregisterNFInstanceMock: func() (*models.ProblemDetails, error) {
				isSendDeregisterNFInstanceCalled = true
				return nil, nil
			},
		},
		{
			name: "ErrorInDeregisterNFInstance",
			sendDeregisterNFInstanceMock: func() (*models.ProblemDetails, error) {
				isSendDeregisterNFInstanceCalled = true
				return nil, errors.New("mock error")
			},
		},
		{
			name: "ProblemDetailsInDeregisterNFInstance",
			sendDeregisterNFInstanceMock: func() (*models.ProblemDetails, error) {
				isSendDeregisterNFInstanceCalled = true
				return &models.ProblemDetails{}, nil
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			KeepAliveTimer = time.NewTimer(60 * time.Second)
			isRegisterNFCalled := false
			isSendDeregisterNFInstanceCalled = false
			originalSendDeregisterNFInstance := consumer.SendDeregisterNFInstance
			originalRegisterNF := registerNF
			defer func() {
				consumer.SendDeregisterNFInstance = originalSendDeregisterNFInstance
				registerNF = originalRegisterNF
				if KeepAliveTimer != nil {
					KeepAliveTimer.Stop()
				}
			}()

			consumer.SendDeregisterNFInstance = tc.sendDeregisterNFInstanceMock

			registerNF = func(ctx context.Context) {
				isRegisterNFCalled = true
			}

			HandleNewConfig([]models.PlmnId{})

			if KeepAliveTimer != nil {
				t.Errorf("expected KeepAliveTimer to be nil after stopKeepAliveTimer")
			}

			if !isSendDeregisterNFInstanceCalled {
				t.Errorf("expected SendDeregisterNFInstance to be called")
			}

			if isRegisterNFCalled {
				t.Errorf("expected registerNF not to be called")
			}

		})
	}
}

func TestHandleNewConfig_ConfigChanged_registerNFFails(t *testing.T) {
	originalSendRegisterNFInstance := consumer.SendRegisterNFInstance
	defer func() {
		consumer.SendRegisterNFInstance = originalSendRegisterNFInstance
		if KeepAliveTimer != nil {
			KeepAliveTimer.Stop()
		}
	}()

	consumer.SendRegisterNFInstance = func(nrfUri string, nfInstanceId string, profile models.NfProfile) (models.NfProfile, string, string, error) {
		profile.HeartBeatTimer = 60
		return profile, "", "", errors.New("mock error")
	}

	// Initial call: should start first registerNF
	HandleNewConfig([]models.PlmnId{{Mcc: "001", Mnc: "01"}})

	time.Sleep(3 * time.Second) // give registerNF a chance to run

	// Save old context cancel
	registerCtxMutex.Lock()
	oldCancel := registerCancel
	oldContext := registerCtx
	registerCtxMutex.Unlock()

	if oldCancel == nil {
		t.Fatal("expected registerCancel to be set")
	}

	// Second config update: should cancel previous context and start a new one
	HandleNewConfig([]models.PlmnId{{Mcc: "001", Mnc: "02"}})

	time.Sleep(2 * time.Second) // give registerNF and cancel time

	select {
	case <-oldContext.Done():
		// expected
	default:
		t.Error("expected old context to be cancelled")
	}

	select {
	case <-registerCtx.Done():
		t.Error("expected context to be running")
	default:
		// expected
	}

}

func TestHandleNewConfig_ConfigChanged_registerNFSuccess_startsTimer(t *testing.T) {
	originalSendRegisterNFInstance := consumer.SendRegisterNFInstance
	defer func() {
		consumer.SendRegisterNFInstance = originalSendRegisterNFInstance
		if KeepAliveTimer != nil {
			KeepAliveTimer.Stop()
		}
	}()

	consumer.SendRegisterNFInstance = func(nrfUri string, nfInstanceId string, profile models.NfProfile) (models.NfProfile, string, string, error) {
		profile.HeartBeatTimer = 60
		return profile, "", "", nil
	}

	// Initial call: should start first registerNF
	HandleNewConfig([]models.PlmnId{{Mcc: "001", Mnc: "01"}})

	time.Sleep(3 * time.Second) // give registerNF a chance to run

	select {
	case <-registerCtx.Done(): // correct this
		//expected
	default:
		t.Error("expected context to be running")
	}

	if KeepAliveTimer == nil {
		t.Error("expected KeepAliveTimer to be initialized by startKeepAliveTimer")
	}

}
