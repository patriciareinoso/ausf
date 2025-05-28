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
	"testing"
	"time"

	"github.com/go-playground/assert/v2"
	"github.com/omec-project/ausf/consumer"
	"github.com/omec-project/openapi/Nnrf_NFDiscovery"
	"github.com/omec-project/openapi/models"
)

func TestRegisterNF(t *testing.T) {
	origRegisterNFInstance := consumer.SendRegisterNFInstance
	origSearchNFInstances := consumer.SendSearchNFInstances
	origUpdateNFInstance := consumer.SendUpdateNFInstance
	defer func() {
		consumer.SendRegisterNFInstance = origRegisterNFInstance
		consumer.SendSearchNFInstances = origSearchNFInstances
		consumer.SendUpdateNFInstance = origUpdateNFInstance
	}()
	t.Logf("test case TestRegisterNF")
	var prof models.NfProfile
	consumer.SendRegisterNFInstance = func(nrfUri string, nfInstanceId string, profile models.NfProfile) (models.NfProfile, string, string, error) {
		prof = profile
		prof.HeartBeatTimer = 1
		t.Logf("test RegisterNFInstance called")
		return prof, "", "", nil
	}
	consumer.SendSearchNFInstances = func(nrfUri string, targetNfType, requestNfType models.NfType, param *Nnrf_NFDiscovery.SearchNFInstancesParamOpts) (models.SearchResult, error) {
		t.Logf("test SearchNFInstance called")
		return models.SearchResult{}, nil
	}
	consumer.SendUpdateNFInstance = func(patchItem []models.PatchItem) (nfProfile models.NfProfile, problemDetails *models.ProblemDetails, err error) {
		return prof, nil, nil
	}

	go registerNF(context.TODO())
	time.Sleep(5 * time.Second)
	assert.Equal(t, KeepAliveTimer != nil, true)

	time.Sleep(1 * time.Second)
	assert.Equal(t, KeepAliveTimer == nil, true)

}
