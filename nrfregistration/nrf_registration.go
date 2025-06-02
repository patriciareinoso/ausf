// SPDX-FileCopyrightText: 2025 Canonical Ltd
// SPDX-FileCopyrightText: 2024 Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nrfregistration

import (
	"context"
	"sync"
	"time"

	"github.com/omec-project/ausf/consumer"
	ausfContext "github.com/omec-project/ausf/context"
	"github.com/omec-project/ausf/logger"
	"github.com/omec-project/openapi/models"
)

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

var (
	registerCtx      context.Context
	registerCancel   context.CancelFunc
	registerCtxMutex sync.Mutex
)

const DEFAULT_HEARTBEAT_TIMER int32 = 60

func startKeepAliveTimer(nfProfile models.NfProfile) {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	stopKeepAliveTimer()
	heartbeatTimer := DEFAULT_HEARTBEAT_TIMER
	if nfProfile.HeartBeatTimer != 0 {
		heartbeatTimer = nfProfile.HeartBeatTimer
	}
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls heartbeatNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(heartbeatTimer)*time.Second, heartbeatNF)
	logger.NrfRegistrationLog.Infof("started heartbeat timer: %v sec", heartbeatTimer)
}

func stopKeepAliveTimer() {
	if KeepAliveTimer != nil {
		KeepAliveTimer.Stop()
		KeepAliveTimer = nil
		logger.NrfRegistrationLog.Infoln("stopped heartbeat timer")
	}
}

func buildAndSendRegisterNFInstance() (models.NfProfile, error) {
	self := ausfContext.GetSelf()
	profile, err := consumer.BuildNFInstance(self)
	if err != nil {
		return profile, err
	}

	profile, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	logger.NrfRegistrationLog.Infoln("AUSF Profile Registering sent to NRF")
	return profile, err
}

// heartbeatNF is the callback function, this is called when keepalivetimer elapsed
func heartbeatNF() {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()

	if KeepAliveTimer == nil {
		logger.NrfRegistrationLog.Warnln("heartbeat timer has been stopped")
		return
	}

	patchItem := []models.PatchItem{
		{
			Op:    "replace",
			Path:  "/nfStatus",
			Value: "REGISTERED",
		},
	}
	nfProfile, problemDetails, err := consumer.SendUpdateNFInstance(patchItem)

	if shouldRegister(problemDetails, err) {
		nfProfile, err = buildAndSendRegisterNFInstance()
		if err != nil {
			logger.NrfRegistrationLog.Errorf("AUSF register to NRF Error[%s]", err.Error())
		}
	}

	heartBeatTimer := DEFAULT_HEARTBEAT_TIMER
	if nfProfile.HeartBeatTimer != 0 {
		heartBeatTimer = nfProfile.HeartBeatTimer
	}
	// restart timer with received HeartBeatTimer value
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, heartbeatNF)
	logger.NrfRegistrationLog.Debugf("restarted heartbeat timer: %v sec", heartBeatTimer)
}

func shouldRegister(problemDetails *models.ProblemDetails, err error) bool {
	if problemDetails != nil {
		logger.NrfRegistrationLog.Warnf("AUSF update to NRF ProblemDetails[%v]", problemDetails)
		status := problemDetails.Status
		return (status/100) == 5 || status == 404 || status == 400
	}
	if err != nil {
		logger.NrfRegistrationLog.Warnf("AUSF update to NRF Error[%s]", err.Error())
		return true
	}
	return false
}

var registerNF = func(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logger.PollConfigLog.Infoln("register AUSF instance to NRF cancelled due to new configuration")
			return
		default:
			ausfContext := ausfContext.GetSelf()
			profile, err := consumer.BuildNFInstance(ausfContext)
			if err != nil {
				logger.NrfRegistrationLog.Warnln("build AUSF profile failed", err)
			}
			profile, _, ausfContext.NfId, err = consumer.SendRegisterNFInstance(ausfContext.NrfUri, ausfContext.NfId, profile)
			if err != nil {
				logger.NrfRegistrationLog.Errorf("register AUSF instance to NRF error[%s]", err.Error())
				time.Sleep(2 * time.Second)
				continue
			}
			logger.CfgLog.Infoln("register AUSF instance to NRF with updated profile succeeded")
			startKeepAliveTimer(profile)
			return
		}
	}
}

var deregisterNF = func() {
	KeepAliveTimerMutex.Lock()
	stopKeepAliveTimer()
	KeepAliveTimerMutex.Unlock()
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if err != nil {
		logger.NrfRegistrationLog.Warnln("deregister instance from NRF failed:", err.Error())
		return
	}
	if problemDetails != nil {
		logger.NrfRegistrationLog.Warnln("deregister instance from NRF failed:", problemDetails)
		return
	}
	logger.NrfRegistrationLog.Infoln("deregister instance from NRF successful")
}

var HandleNewConfig = func(newPlmnConfig []models.PlmnId) {
	registerCtxMutex.Lock()
	defer registerCtxMutex.Unlock()

	if registerCancel != nil {
		registerCancel()
		logger.NrfRegistrationLog.Infoln("registration context cancelled")
	}

	if len(newPlmnConfig) == 0 {
		logger.NrfRegistrationLog.Debugln("PLMN config is empty. AUSF will degister")
		deregisterNF()
	} else {
		logger.NrfRegistrationLog.Debugln("PLMN config is not empty. AUSF will update registration")
		// Create new cancellable context for this registration
		registerCtx, registerCancel = context.WithCancel(context.Background())
		go registerNF(registerCtx)
	}
}
