// SPDX-FileCopyrightText: 2025 Canonical Ltd
// SPDX-FileCopyrightText: 2024 Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nrfregistration

import (
	"sync"
	"time"

	"github.com/omec-project/ausf/consumer"
	"github.com/omec-project/ausf/context"
	"github.com/omec-project/ausf/logger"
	"github.com/omec-project/openapi/models"
)

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
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
	// AfterFunc starts timer and waits for KeepAliveTimer to elapse and then calls UpdateNF function
	KeepAliveTimer = time.AfterFunc(time.Duration(heartbeatTimer)*time.Second, UpdateNF)
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
	self := context.GetSelf()
	profile, err := consumer.BuildNFInstance(self)
	if err != nil {
		logger.NrfRegistrationLog.Errorf("build AUSF Profile Error: %v", err)
		return profile, err
	}
	logger.NrfRegistrationLog.Infof("AUSF Profile Registering to NRF: %v", profile)
	// Indefinite attempt to register until success
	profile, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	return profile, err
}

// UpdateNF is the callback function, this is called when keepalivetimer elapsed
func UpdateNF() {
	KeepAliveTimerMutex.Lock()
	defer KeepAliveTimerMutex.Unlock()
	if KeepAliveTimer == nil {
		logger.NrfRegistrationLog.Warnln("KeepAlive timer has been stopped") ////////
		return
	}
	// setting default value 60 sec
	heartBeatTimer := DEFAULT_HEARTBEAT_TIMER
	pitem := models.PatchItem{
		Op:    "replace",
		Path:  "/nfStatus",
		Value: "REGISTERED",
	}
	var patchItem []models.PatchItem
	patchItem = append(patchItem, pitem)
	nfProfile, problemDetails, err := consumer.SendUpdateNFInstance(patchItem)
	if problemDetails != nil {
		logger.NrfRegistrationLog.Errorf("AUSF update to NRF ProblemDetails[%v]", problemDetails)
		// 5xx response from NRF, 404 Not Found, 400 Bad Request
		if (problemDetails.Status/100) == 5 ||
			problemDetails.Status == 404 || problemDetails.Status == 400 {
			// register with NRF full profile
			nfProfile, err = buildAndSendRegisterNFInstance()
			if err != nil {
				logger.NrfRegistrationLog.Errorf("AUSF register to NRF Error[%s]", err.Error())
			}
		}
	} else if err != nil {
		logger.NrfRegistrationLog.Errorf("AUSF update to NRF Error[%s]", err.Error())
		nfProfile, err = buildAndSendRegisterNFInstance()
		if err != nil {
			logger.NrfRegistrationLog.Errorf("AUSF register to NRF Error[%s]", err.Error())
		}
	}

	if nfProfile.HeartBeatTimer != 0 {
		heartBeatTimer = nfProfile.HeartBeatTimer
	}
	logger.NrfRegistrationLog.Debugf("restarted KeepAlive Timer: %v sec", heartBeatTimer)
	// restart timer with received HeartBeatTimer value
	KeepAliveTimer = time.AfterFunc(time.Duration(heartBeatTimer)*time.Second, UpdateNF)
}

func RegisterNF() {
	self := context.GetSelf()
	profile, err := consumer.BuildNFInstance(self)
	if err != nil {
		logger.NrfRegistrationLog.Errorln("build AUSF Profile Error")
	}
	var prof models.NfProfile
	prof, _, self.NfId, err = consumer.SendRegisterNFInstance(self.NrfUri, self.NfId, profile)
	if err != nil {
		logger.NrfRegistrationLog.Errorf("AUSF register to NRF Error[%s]", err.Error())
	} else {
		startKeepAliveTimer(prof)
		logger.CfgLog.Infoln("sent Register NF Instance with updated profile")
	}
}

func DeregisterNF() {
	KeepAliveTimerMutex.Lock()
	stopKeepAliveTimer()
	KeepAliveTimerMutex.Unlock()
	problemDetails, err := consumer.SendDeregisterNFInstance()
	if problemDetails != nil {
		logger.NrfRegistrationLog.Errorf("deregister Instance to NRF failed, Problem: [+%v]", problemDetails)
	}
	if err != nil {
		logger.NrfRegistrationLog.Errorf("deregister Instance to NRF Error[%s]", err.Error())
		return
	}
	logger.NrfRegistrationLog.Infoln("deregister from NRF successfully")
}
