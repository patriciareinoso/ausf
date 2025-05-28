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
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls updateNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(heartbeatTimer)*time.Second, updateNF)
	logger.NrfRegistrationLog.Infof("started KeepAlive Timer: %v sec", heartbeatTimer)
}

func stopKeepAliveTimer() {
	if KeepAliveTimer != nil {
		KeepAliveTimer.Stop()
		KeepAliveTimer = nil
		logger.NrfRegistrationLog.Infoln("stopped KeepAlive Timer")
	}
}

func buildAndSendRegisterNFInstance() (models.NfProfile, error) {
	self := ausfContext.GetSelf()
	profile, err := consumer.BuildNFInstance(self)
	if err != nil {
		logger.NrfRegistrationLog.Errorf("build AUSF Profile Error: %v", err)
		return profile, err
	}
	logger.NrfRegistrationLog.Infof("AUSF Profile Registering to NRF: %v", profile)
	profile, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	return profile, err
}

// updateNF is the callback function, this is called when keepalivetimer elapsed
func updateNF() {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()

	if KeepAliveTimer == nil {
		logger.NrfRegistrationLog.Warnln("KeepAlive timer has been stopped")
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
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, updateNF)
	logger.NrfRegistrationLog.Debugf("restarted KeepAlive Timer: %v sec", heartBeatTimer)
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

func registerNF(ctx context.Context) {
	// should stop heartbeat?
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
		}
	}
}

func deregisterNF() {
	KeepAliveTimerMutex.Lock()
	stopKeepAliveTimer()
	KeepAliveTimerMutex.Unlock()
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if err != nil {
		logger.NrfRegistrationLog.Warnln("deregister instance from NRF failed", err.Error())
		return
	}
	if problemDetails != nil {
		logger.NrfRegistrationLog.Warnln("deregister instance from NRF failed", problemDetails)
		return
	}
	logger.NrfRegistrationLog.Infoln("deregister instance from NRF successful")
}

var HandleNewConfig = func(newPlmnConfig []models.PlmnId) {
	registerCtxMutex.Lock()
	defer registerCtxMutex.Unlock()

	if registerCancel != nil {
		registerCancel()
	}

	if len(newPlmnConfig) == 0 {
		logger.PollConfigLog.Debugln("PLMN config is empty")
		deregisterNF()
	} else {
		logger.PollConfigLog.Debugln("PLMN config is not empty")
		// Create new cancellable context for this registration
		registerCtx, registerCancel = context.WithCancel(context.Background())
		go registerNF(registerCtx)
	}
}
