// SPDX-FileCopyrightText: 2025 Canonical Ltd.
//
// SPDX-License-Identifier: Apache-2.0
//
/*
 * NF Polling Unit Testcases
 *
 */

package polling

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/omec-project/ausf/context"
	"github.com/omec-project/ausf/factory"
	"github.com/omec-project/openapi/models"
)

func TestHandlePolledPlmnConfig_UpdateConfig(t *testing.T) {
	context := &context.AUSFContext{
		PlmnList: []models.PlmnId{
			{Mcc: "001", Mnc: "01"},
		},
	}

	newConfig := []models.PlmnId{
		{Mcc: "001", Mnc: "02"},
	}

	handlePolledPlmnConfig(context, newConfig)

	if !reflect.DeepEqual(context.PlmnList, newConfig) {
		t.Errorf("Expected PLMN config to be updated to %v, got %v", newConfig, context.PlmnList)
	}
}

func TestHandlePolledPlmnConfig_EmptyList(t *testing.T) {
	context := &context.AUSFContext{
		PlmnList: []models.PlmnId{{Mcc: "001", Mnc: "01"}},
	}

	newConfig := []models.PlmnId{}

	handlePolledPlmnConfig(context, newConfig)

	if len(context.PlmnList) != 0 {
		t.Errorf("Expected empty PLMN config, go %v", context.PlmnList)
	}
}

func TestHandlePolledPlmnConfig_ConfigDidNotChanged(t *testing.T) {
	original := []models.PlmnId{
		{Mcc: "001", Mnc: "01"},
	}
	context := &context.AUSFContext{
		PlmnList: original,
	}

	newConfig := []models.PlmnId{
		{Mcc: "001", Mnc: "01"},
	}

	handlePolledPlmnConfig(context, newConfig)

	if !reflect.DeepEqual(context.PlmnList, original) {
		t.Errorf("Expected PLMN list to remain unchanged, got %v", context.PlmnList)
	}
}

func TestFetchPlmnConfig_Success(t *testing.T) {
	expectedPlmnConfig := []models.PlmnId{{Mcc: "001", Mnc: "01"}}
	handler := func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(expectedPlmnConfig)
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	factory.AusfConfig = factory.Config{
		Configuration: &factory.Configuration{
			WebuiUri: server.URL,
		},
	}

	fetchedConfig, err := fetchPlmnConfig()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !reflect.DeepEqual(fetchedConfig, expectedPlmnConfig) {
		t.Errorf("expected %v, got %v", expectedPlmnConfig, fetchedConfig)
	}
}

func TestFetchPlmnConfig_BadJSON(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "not-json")
	}
	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	factory.AusfConfig = factory.Config{
		Configuration: &factory.Configuration{
			WebuiUri: server.URL,
		},
	}

	_, err := fetchPlmnConfig()
	if err == nil {
		t.Error("expected error for bad JSON")
	}
}
