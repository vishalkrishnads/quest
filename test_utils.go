// Copyright (c) 2023 Cloudnatively Services Pvt Ltd
//
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	sleepDuration = 2 * time.Second
)

type StreamHotTier struct {
	Size                string  `json:"size"`
	UsedSize            *string `json:"used_size,omitempty"`
	AvailableSize       *string `json:"available_size,omitempty"`
	OldestDateTimeEntry *string `json:"oldest_date_time_entry,omitempty"`
}

type StreamInfo struct {
	CreatedAt          string  `json:"created-at"`
	FirstEventAt       *string `json:"first-event-at"`
	CacheEnabled       *bool   `json:"cache_enabled"`
	TimePartition      *string `json:"time_partition"`
	TimePartitionLimit *string `json:"time_partition_limit"`
	CustomPartition    *string `json:"custom_partition"`
	StaticSchemaFlag   *string `json:"static_schema_flag"`
}

func flogStreamFields() []string {
	return []string{
		"p_timestamp",
		"p_tags",
		"p_metadata",
		"host",
		"'user-identifier'",
		"datetime",
		"method",
		"request",
		"protocol",
		"status",
		"bytes",
		"referer",
	}
}

func readAsString(body io.Reader) string {
	r, _ := io.ReadAll(body)
	return string(r)
}

func readJsonBody[T any](body io.Reader) (res T, err error) {
	r, _ := io.ReadAll(body)
	err = json.Unmarshal(r, &res)
	return
}

func Sleep() {
	time.Sleep(sleepDuration)
}

func CreateStream(t *testing.T, client HTTPClient, stream string) {
	req, _ := client.NewRequest("PUT", "logstream/"+stream, nil)
	response, err := client.Do(req)
	body := readAsString(response.Body)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s with response: %s", response.Status, body)
}

func CreateStreamWithHeader(t *testing.T, client HTTPClient, stream string, header map[string]string) {
	req, _ := client.NewRequest("PUT", "logstream/"+stream, nil)
	for k, v := range header {
		req.Header.Add(k, v)
	}
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s", response.Status)
}

func CreateStreamWithSchemaBody(t *testing.T, client HTTPClient, stream string, header map[string]string) {
	var schema_payload string = `{
		"fields":[
		 {
			 "name": "source_time",
			 "data_type": "string"
		 },
		 {
			 "name": "level",
			 "data_type": "string"
		 },
		 {
			 "name": "message",
			 "data_type": "string"
		 },
         {
			 "name": "version",
			 "data_type": "string"
		 },
		 {
			 "name": "user_id",
			 "data_type": "int"
		 },
		 {
			 "name": "device_id",
			 "data_type": "int"
		 },
		 {
			 "name": "session_id",
			 "data_type": "string"
		 },
		 {
			 "name": "os",
			 "data_type": "string"
		 },
		 {
			 "name": "host",
			 "data_type": "string"
		 },
		 {
			 "name": "uuid",
			 "data_type": "string"
		 },
		 {
			 "name": "location",
			 "data_type": "string"
		 },
		 {
			 "name": "timezone",
			 "data_type": "string"
		 },
		 {
			 "name": "user_agent",
			 "data_type": "string"
		 },
		 {
			 "name": "runtime",
			 "data_type": "string"
		 },
		 {
			 "name": "request_body",
			 "data_type": "string"
		 },
		 {
			 "name": "status_code",
			 "data_type": "int"
		 },
		 {
			 "name": "response_time",
			 "data_type": "int"
		 },
		 {
			 "name": "process_id",
			 "data_type": "int"
		 },
		 {
			 "name": "app_meta",
			 "data_type": "string"
		 }
	 ]
	 }`
	req, _ := client.NewRequest("PUT", "logstream/"+stream, bytes.NewBufferString(schema_payload))
	for k, v := range header {
		req.Header.Add(k, v)
	}
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s", response.Status)
}

func DeleteStream(t *testing.T, client HTTPClient, stream string) {
	req, _ := client.NewRequest("DELETE", "logstream/"+stream, nil)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s", response.Status)
}

func RunFlog(t *testing.T, client HTTPClient, stream string) {
	cmd := exec.Command("flog", "-f", "json", "-n", "50")
	var out strings.Builder
	cmd.Stdout = &out
	err := cmd.Run()
	require.NoErrorf(t, err, "Failed to run flog: %s", err)

	for _, obj := range strings.SplitN(out.String(), "\n", 50) {
		var payload strings.Builder
		payload.WriteRune('[')
		payload.WriteString(obj)
		payload.WriteRune(']')

		req, _ := client.NewRequest("POST", "ingest", bytes.NewBufferString(payload.String()))
		req.Header.Add("X-P-Stream", stream)
		response, err := client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s resp %s", response.Status, readAsString(response.Body))
	}
}

func IngestOneEventWithTimePartition_TimeStampMismatch(t *testing.T, client HTTPClient, stream string) {
	var test_payload string = `{"source_time":"2024-03-26T18:08:00.434Z","level":"info","message":"Application is failing","version":"1.2.0","user_id":13912,"device_id":4138,"session_id":"abc","os":"Windows","host":"112.168.1.110","location":"ngeuprqhynuvpxgp","request_body":"rnkmffyawtdcindtrdqruyxbndbjpfsptzpwtujbmkwcqastmxwbvjwphmyvpnhordwljnodxhtvpjesjldtifswqbpyuhlcytmm","status_code":300,"app_meta":"ckgpibhmlusqqfunnpxbfxbc", "new_field_added_by":"ingestor 8020"}`
	req, _ := client.NewRequest("POST", "ingest", bytes.NewBufferString(test_payload))
	req.Header.Add("X-P-Stream", stream)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 400, response.StatusCode, "Server returned http code: %s resp %s", response.Status, readAsString(response.Body))
}

func IngestOneEventWithTimePartition_NoTimePartitionInLog(t *testing.T, client HTTPClient, stream string) {
	var test_payload string = `{"level":"info","message":"Application is failing","version":"1.2.0","user_id":13912,"device_id":4138,"session_id":"abc","os":"Windows","host":"112.168.1.110","location":"ngeuprqhynuvpxgp","request_body":"rnkmffyawtdcindtrdqruyxbndbjpfsptzpwtujbmkwcqastmxwbvjwphmyvpnhordwljnodxhtvpjesjldtifswqbpyuhlcytmm","status_code":300,"app_meta":"ckgpibhmlusqqfunnpxbfxbc", "new_field_added_by":"ingestor 8020"}`
	req, _ := client.NewRequest("POST", "ingest", bytes.NewBufferString(test_payload))
	req.Header.Add("X-P-Stream", stream)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 400, response.StatusCode, "Server returned http code: %s resp %s", response.Status, readAsString(response.Body))
}

func IngestOneEventWithTimePartition_IncorrectDateTimeFormatTimePartitionInLog(t *testing.T, client HTTPClient, stream string) {
	var test_payload string = `{"source_time":"2024-03-26", "level":"info","message":"Application is failing","version":"1.2.0","user_id":13912,"device_id":4138,"session_id":"abc","os":"Windows","host":"112.168.1.110","location":"ngeuprqhynuvpxgp","request_body":"rnkmffyawtdcindtrdqruyxbndbjpfsptzpwtujbmkwcqastmxwbvjwphmyvpnhordwljnodxhtvpjesjldtifswqbpyuhlcytmm","status_code":300,"app_meta":"ckgpibhmlusqqfunnpxbfxbc", "new_field_added_by":"ingestor 8020"}`
	req, _ := client.NewRequest("POST", "ingest", bytes.NewBufferString(test_payload))
	req.Header.Add("X-P-Stream", stream)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 400, response.StatusCode, "Server returned http code: %s resp %s", response.Status, readAsString(response.Body))
}

func IngestOneEventForStaticSchemaStream_NewFieldInLog(t *testing.T, client HTTPClient, stream string) {
	var test_payload string = `{"source_time":"2024-03-26", "level":"info","message":"Application is failing","version":"1.2.0","user_id":13912,"device_id":4138,"session_id":"abc","os":"Windows","host":"112.168.1.110","location":"ngeuprqhynuvpxgp","request_body":"rnkmffyawtdcindtrdqruyxbndbjpfsptzpwtujbmkwcqastmxwbvjwphmyvpnhordwljnodxhtvpjesjldtifswqbpyuhlcytmm","status_code":300,"app_meta":"ckgpibhmlusqqfunnpxbfxbc", "new_field_added_by":"ingestor 8020"}`
	req, _ := client.NewRequest("POST", "ingest", bytes.NewBufferString(test_payload))
	req.Header.Add("X-P-Stream", stream)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 400, response.StatusCode, "Server returned http code: %s resp %s", response.Status, readAsString(response.Body))
}

func IngestOneEventForStaticSchemaStream_SameFieldsInLog(t *testing.T, client HTTPClient, stream string) {
	var test_payload string = `{"source_time":"2024-03-26", "level":"info","message":"Application is failing","version":"1.2.0","user_id":13912,"device_id":4138,"session_id":"abc","os":"Windows","host":"112.168.1.110","location":"ngeuprqhynuvpxgp","request_body":"rnkmffyawtdcindtrdqruyxbndbjpfsptzpwtujbmkwcqastmxwbvjwphmyvpnhordwljnodxhtvpjesjldtifswqbpyuhlcytmm","status_code":300,"app_meta":"ckgpibhmlusqqfunnpxbfxbc"}`
	req, _ := client.NewRequest("POST", "ingest", bytes.NewBufferString(test_payload))
	req.Header.Add("X-P-Stream", stream)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s resp %s", response.Status, readAsString(response.Body))
}

func QueryLogStreamCount(t *testing.T, client HTTPClient, stream string, count uint64) string {
	// Query last 30 minutes of data only
	endTime := time.Now().Add(time.Second).Format(time.RFC3339Nano)
	startTime := time.Now().Add(-30 * time.Minute).Format(time.RFC3339Nano)

	query := map[string]interface{}{
		"query":     "select count(*) as count from " + stream,
		"startTime": startTime,
		"endTime":   endTime,
	}
	queryJSON, _ := json.Marshal(query)
	req, _ := client.NewRequest("POST", "query", bytes.NewBuffer(queryJSON))
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	expected := fmt.Sprintf(`[{"count":%d}]`, count)
	require.Equalf(t, expected, body, "Query count incorrect; Expected %s, Actual %s", expected, body)
	return body
}

func QueryLogStreamCount_Historical(t *testing.T, client HTTPClient, stream string, count uint64) {
	// Query last 30 minutes of data only
	now := time.Now()
	startTime := now.AddDate(0, 0, -33).Format(time.RFC3339Nano)
	endTime := now.AddDate(0, 0, -27).Format(time.RFC3339Nano)

	query := map[string]interface{}{
		"query":     "select count(*) as count from " + stream,
		"startTime": startTime,
		"endTime":   endTime,
	}
	queryJSON, _ := json.Marshal(query)
	req, _ := client.NewRequest("POST", "query", bytes.NewBuffer(queryJSON))
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	expected := fmt.Sprintf(`[{"count":%d}]`, count)
	require.Equalf(t, expected, body, "Query count incorrect; Expected %s, Actual %s", expected, body)
}

func QueryTwoLogStreamCount(t *testing.T, client HTTPClient, stream1 string, stream2 string, count uint64) {
	// Query last 30 minutes of data only
	endTime := time.Now().Add(time.Second).Format(time.RFC3339Nano)
	startTime := time.Now().Add(-30 * time.Minute).Format(time.RFC3339Nano)

	query := map[string]interface{}{
		"query":     fmt.Sprintf("select sum(c) as count from (select count(*) as c from %s union all select count(*) as c from %s)", stream1, stream2),
		"startTime": startTime,
		"endTime":   endTime,
	}
	queryJSON, _ := json.Marshal(query)
	req, _ := client.NewRequest("POST", "query", bytes.NewBuffer(queryJSON))
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	expected := fmt.Sprintf(`[{"count":%d}]`, count)
	require.Equalf(t, expected, body, "Query count incorrect; Expected %s, Actual %s", expected, body)
}

func AssertQueryOK(t *testing.T, client HTTPClient, query string, args ...any) {
	// Query last 30 minutes of data only
	endTime := time.Now().Add(time.Second).Format(time.RFC3339Nano)
	startTime := time.Now().Add(-30 * time.Minute).Format(time.RFC3339Nano)

	var finalQuery string
	if len(args) == 0 {
		finalQuery = query
	} else {
		finalQuery = fmt.Sprintf(query, args...)
	}

	queryJSON, _ := json.Marshal(map[string]interface{}{
		"query":     finalQuery,
		"startTime": startTime,
		"endTime":   endTime,
	})

	req, _ := client.NewRequest("POST", "query", bytes.NewBuffer(queryJSON))
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
}

func AssertStreamSchema(t *testing.T, client HTTPClient, stream string, schema string) {
	req, _ := client.NewRequest("GET", "logstream/"+stream+"/schema", nil)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	require.JSONEq(t, schema, body, "Get schema response doesn't match with expected schema")
}

func CreateRole(t *testing.T, client HTTPClient, name string, role string) {
	req, _ := client.NewRequest("PUT", "role/"+name, strings.NewReader(role))
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))
}

func AssertRole(t *testing.T, client HTTPClient, name string, role string) {
	req, _ := client.NewRequest("GET", "role/"+name, nil)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	require.JSONEq(t, role, body, "Get role response doesn't match with retention config returned")
}

func CreateUser(t *testing.T, client HTTPClient, user string) string {
	req, _ := client.NewRequest("POST", "user/"+user, nil)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	return body
}

func CreateUserWithRole(t *testing.T, client HTTPClient, user string, roles []string) string {
	payload, _ := json.Marshal(roles)
	req, _ := client.NewRequest("POST", "user/"+user, bytes.NewBuffer(payload))
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	return body
}

func AssignRolesToUser(t *testing.T, client HTTPClient, user string, roles []string) {
	payload, _ := json.Marshal(roles)
	req, _ := client.NewRequest("PUT", "user/"+user+"/role", bytes.NewBuffer(payload))
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))
}

func AssertUserRole(t *testing.T, client HTTPClient, user string, roleName, roleBody string) {
	req, _ := client.NewRequest("GET", "user/"+user+"/role", nil)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	userRoleBody := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, userRoleBody)
	expectedRoleBody := fmt.Sprintf(`{"%s":%s}`, roleName, roleBody)
	require.JSONEq(t, userRoleBody, expectedRoleBody, "Get user role response doesn't match with expected role")
}

func RegenPassword(t *testing.T, client HTTPClient, user string) string {
	req, _ := client.NewRequest("POST", "user/"+user+"/generate-new-password", nil)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	return body
}

func SetUserRole(t *testing.T, client HTTPClient, user string, roles []string) {
	payload, _ := json.Marshal(roles)
	req, _ := client.NewRequest("PUT", "user/"+user+"/role", bytes.NewBuffer(payload))
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))
}

func DeleteUser(t *testing.T, client HTTPClient, user string) {
	req, _ := client.NewRequest("DELETE", "user/"+user, nil)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))
}

func DeleteRole(t *testing.T, client HTTPClient, roleName string) {
	req, _ := client.NewRequest("DELETE", "role/"+roleName, nil)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))
}

func SetDefaultRole(t *testing.T, client HTTPClient, roleName string) {
	payload, _ := json.Marshal(roleName)
	req, _ := client.NewRequest("PUT", "role/default", bytes.NewBuffer(payload))
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))
}

func AssertDefaultRole(t *testing.T, client HTTPClient, roleName string) {
	req, _ := client.NewRequest("GET", "role/default", nil)
	response, err := client.Do(req)
	require.NoErrorf(t, err, "Request failed: %s", err)
	body := readAsString(response.Body)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	require.Equalf(t, roleName, body, "Get default role response doesn't match with expected role")
}

func PutSingleEventExpectErr(t *testing.T, client HTTPClient, stream string) {
	payload := `{
		"id": "id;objectId",
		"maxRunDistance": "float;1;20;1",
		"cpf": "cpf",
		"cnpj": "cnpj",
		"pretendSalary": "money",
		"age": "int;20;80",
		"gender": "gender",
		"firstName": "firstName",
		"lastName": "lastName",
		"phone": "maskInt;+55 (83) 9####-####",
		"address": "address",
		"hairColor": "color"
	}`
	req, _ := client.NewRequest("POST", "logstream/"+stream, bytes.NewBufferString(payload))
	response, err := client.Do(req)

	require.NoErrorf(t, err, "Request failed when expected to pass: %s", err)
	require.Equalf(t, 403, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))
}

func PutSingleEvent(t *testing.T, client HTTPClient, stream string) {
	payload := `{
		"id": "id;objectId",
		"maxRunDistance": "float;1;20;1",
		"cpf": "cpf",
		"cnpj": "cnpj",
		"pretendSalary": "money",
		"age": "int;20;80",
		"gender": "gender",
		"firstName": "firstName",
		"lastName": "lastName",
		"phone": "maskInt;+55 (83) 9####-####",
		"address": "address",
		"hairColor": "color"
	}`
	req, _ := client.NewRequest("POST", "logstream/"+stream, bytes.NewBufferString(payload))
	response, err := client.Do(req)

	require.NoErrorf(t, err, "Request failed: %s", err)
	require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))
}

func checkAPIAccess(t *testing.T, client HTTPClient, stream string, role string) {
	switch role {
	case "editor":
		// Check access to non-protected API
		req, _ := client.NewRequest("GET", "liveness", nil)
		response, err := client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

		// Check access to protected API with access
		req, _ = client.NewRequest("GET", "logstream", nil)
		response, err = client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

		// Attempt to call protected API without access
		req, _ = client.NewRequest("DELETE", "logstream/"+stream, nil)
		response, err = client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 403, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

	case "writer":
		// Check access to non-protected API
		req, _ := client.NewRequest("GET", "liveness", nil)
		response, err := client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

		// Check access to protected API with access
		req, _ = client.NewRequest("GET", "logstream", nil)
		response, err = client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

		// Attempt to call protected API without access
		req, _ = client.NewRequest("DELETE", "logstream/"+stream, nil)
		response, err = client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 403, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

	case "reader":
		// Check access to non-protected API
		req, _ := client.NewRequest("GET", "liveness", nil)
		response, err := client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

		// Check access to protected API with access
		req, _ = client.NewRequest("GET", "logstream", nil)
		response, err = client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

		// Attempt to call protected API without access
		req, _ = client.NewRequest("DELETE", "logstream/"+stream, nil)
		response, err = client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 403, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

	case "ingestor":
		// Check access to non-protected API
		req, _ := client.NewRequest("GET", "liveness", nil)
		response, err := client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))

		// Check access to protected API with access
		PutSingleEvent(t, client, stream)

		// Attempt to call protected API without access
		req, _ = client.NewRequest("DELETE", "logstream/"+stream, nil)
		response, err = client.Do(req)
		require.NoErrorf(t, err, "Request failed: %s", err)
		require.Equalf(t, 403, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, readAsString(response.Body))
	}
}

func activateHotTier(t *testing.T, size string, verify bool) (int, string) {
	if size == "" {
		size = "20 GiB" // default hot tier size
	}

	payload := StreamHotTier{
		Size: size,
	}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := NewGlob.QueryClient.NewRequest("PUT", "logstream/"+NewGlob.Stream+"/hottier", bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")
	response, err := NewGlob.QueryClient.Do(req)
	body := readAsString(response.Body)

	if verify {
		if NewGlob.IngestorUrl.String() != "" {
			require.Equalf(t, 200, response.StatusCode, "Server returned unexpected http code: %s and response: %s", response.Status, body)
			require.NoErrorf(t, err, "Activating hot tier failed in distributed mode: %s", err)
		} else {
			// right now, hot tier is unavailable in standalone so anything other than 200 is fine
			require.NotEqualf(t, 200, response.StatusCode, "Hot tier has been activated in standalone mode: %s and response: %s", response.Status, body)
		}
	}

	return response.StatusCode, body
}

func getHotTierStatus(t *testing.T, shouldFail bool) *StreamHotTier {
	req, err := NewGlob.QueryClient.NewRequest("GET", "logstream/"+NewGlob.Stream+"/hottier", nil)
	require.NoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")

	response, err := NewGlob.QueryClient.Do(req)
	require.NoError(t, err, "Failed to execute GET /hottier")
	defer response.Body.Close()

	body := readAsString(response.Body)

	if shouldFail {
		require.NotEqualf(t, 200, response.StatusCode, "Hot tier was expected to fail but succeeded with body: %s", body)
		return &StreamHotTier{Size: "0"}
	} else {
		require.Equalf(t, 200, response.StatusCode, "GET hot tier failed with status code: %d & body: %s", response.StatusCode, body)
	}

	var hotTierStatus StreamHotTier
	err = json.Unmarshal([]byte(body), &hotTierStatus)
	require.NoError(t, err, "The response from GET /hottier isn't of expected schema: %s", body)

	return &hotTierStatus
}

func disableHotTier(t *testing.T, shouldFail bool) {
	req, _ := NewGlob.QueryClient.NewRequest("DELETE", "logstream/"+NewGlob.Stream+"/hottier", nil)
	response, err := NewGlob.QueryClient.Do(req)
	body := readAsString(response.Body)

	if shouldFail {
		require.NotEqualf(t, 200, response.StatusCode, "Non-existent hot tier was disabled with response: %s", body)
	} else {
		require.Equalf(t, 200, response.StatusCode, "Server returned http code: %s and response: %s", response.Status, body)
	}
	require.NoErrorf(t, err, "Disabling hot tier failed: %s", err)
}

func getStreamInfo(t *testing.T) *StreamInfo {
	req, err := NewGlob.QueryClient.NewRequest("GET", "logstream/"+NewGlob.Stream+"/info", nil)
	require.NoError(t, err, "Failed to create request")

	req.Header.Set("Content-Type", "application/json")

	response, err := NewGlob.QueryClient.Do(req)
	require.NoError(t, err, "Failed to execute GET /logstream/{stream_name}/info")
	defer response.Body.Close()

	body := readAsString(response.Body)

	require.Equal(t, 200, response.StatusCode, "GET /logstream/{stream_name}/info failed with status code: %d & body: %s", response.StatusCode, body)

	var streamInfo StreamInfo
	err = json.Unmarshal([]byte(body), &streamInfo)
	require.NoError(t, err, "The response from GET /info isn't of expected schema: %s", body)

	return &streamInfo
}
