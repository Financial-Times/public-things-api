package things

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger"
	"github.com/stretchr/testify/mock"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

const (
	canonicalUUID       = "00000000-0000-002a-0000-00000000002a"
	secondCanonicalUUID = "00000000-0000-002a-0000-00000000002d"
	thirdCanonicalUUID  = "00000000-0000-002a-0000-00000000002f"
	alternateUUID       = "00000000-0000-002a-0000-00000000002b"
	invalidUUID         = "00000000-0000-002a-0000"
)

var testConcept = Thing{ID: canonicalUUID, APIURL: canonicalUUID, Types: []string{}}
var testSecondConcept = Thing{ID: secondCanonicalUUID, APIURL: secondCanonicalUUID, Types: []string{}}
var testThirdConcept = Thing{ID: thirdCanonicalUUID, APIURL: thirdCanonicalUUID, Types: []string{}}
var testRelationships = []string{"testBroader", "testNarrower"}

type mockHTTPClient struct {
	resp       string
	statusCode int
	err        error
}

type testCase struct {
	name         string
	url          string
	clientCode   int
	clientBody   string
	clientError  error
	expectedCode int
	expectedBody string
}

func (mhc *mockHTTPClient) Do(req *http.Request) (resp *http.Response, err error) {
	cb := ioutil.NopCloser(bytes.NewReader([]byte(mhc.resp)))
	return &http.Response{Body: cb, StatusCode: mhc.statusCode}, mhc.err
}

func TestHandlers(t *testing.T) {
	logger.InitLogger("test service", "debug")
	var mockClient mockHTTPClient
	router := mux.NewRouter()

	getThingSuccess := testCase{
		"GetThing - Basic successful request",
		"/things/6773e864-78ab-4051-abc2-f4e9ab423ebb",
		200,
		getCompleteThingAsConcept,
		nil,
		200,
		transformedCompleteThing,
	}

	getThingSuccessWithRelationShip := testCase{
		"GetThing - Basic successful request with relationship parameter",
		"/things/6773e864-78ab-4051-abc2-f4e9ab423ebb?showRelationship=related",
		200,
		getConmpleteThingWithRelationAsConcept,
		nil,
		200,
		transformedCompleteThingWithRelation,
	}

	getThingNotFound := testCase{
		"GetThing - request is not found",
		"/things/6773e864-78ab-4051-abc2-f4e9ab423ebc",
		404,
		"",
		nil,
		404,
		`{"message":"No thing found with uuid 6773e864-78ab-4051-abc2-f4e9ab423ebc."}`,
	}

	getThingWithInvalidUUID := testCase{
		"GetThing - bad request due to invalid uuid",
		"/things/111111111111111111",
		200,
		"",
		nil,
		400,
		"invalid/malformed uuid\n",
	}

	getThingWithConceptsAPIError := testCase{
		"GetThing - Service Unavailable because of concepts api internal server error",
		"/things/6773e864-78ab-4051-abc2-f4e9ab423ebc",
		500,
		"",
		errors.New("Internal Server Error"),
		503,
		`{"message":"Error getting thing with uuid 6773e864-78ab-4051-abc2-f4e9ab423ebc, err=Internal Server Error"}`,
	}

	getThingWithConceptsAPIInvalidResponse := testCase{
		"GetThing - Service Unavailable because of concepts api internal server error",
		"/things/6773e864-78ab-4051-abc2-f4e9ab423ebc",
		200,
		`{"foo":bar}`,
		nil,
		503,
		`{"message":"Error getting thing with uuid 6773e864-78ab-4051-abc2-f4e9ab423ebc, err=invalid character 'b' looking for beginning of value"}`,
	}

	getThingRedirect := testCase{
		"GetThing - redirect",
		"/things/6773e864-78ab-4051-abc2-f4e9ab423ebc",
		200,
		getCompleteThingAsConcept,
		nil,
		301,
		``,
	}

	getThingRedirectWithRelationships := testCase{
		"GetThing - redirect with relationships parameter",
		"/things/6773e864-78ab-4051-abc2-f4e9ab423ebc?showRelationship=narrower",
		200,
		getCompleteThingAsConcept,
		nil,
		301,
		``,
	}

	getThingsWithoutParams := testCase{
		"GetThings - request with no query parameter",
		"/things",
		400,
		"",
		nil,
		400,
		`{"message":"at least one uuid query param should be provided for batch operations"}`,
	}

	getThingsNotFound := testCase{
		"GetThings - request with valid format UUID, but not exist",
		"/things?uuid=6773e864-78ab-4051-abc2-f4e9ab423ebc",
		404,
		"",
		nil,
		200,
		`{"things":{}}`,
	}

	getThingsWithInvalidUUID := testCase{
		"GetThings - request with invalid format UUID",
		"/things?uuid=6773e864-111178ab-4051-abc2-f4e9ab423ebc",
		400,
		"",
		nil,
		400,
		`{"message":"Invalid uuid: 6773e864-111178ab-4051-abc2-f4e9ab423ebc, err: invalid UUID length: 40"}`,
	}

	getThingsWithAlternativeUUID := testCase{
		"GetThings - request with alternative uuid, which returns canonical uuid",
		"/things?uuid=6773e864-78ab-4051-abc2-f4e9ab423ebc",
		200,
		getCompleteThingAsConcept,
		nil,
		200,
		`{"things":{"6773e864-78ab-4051-abc2-f4e9ab423ebc":` + transformedCompleteThing + `}}`,
	}

	getBrand := testCase{
		"",
		"/things/c3e3fe44-93fb-11e8-8f42-da24cd01f044?showRelationship=broader&showRelationship=narrower",
		200,
		brandAsConcept,
		nil,
		200,
		transformedBrand,
	}

	testCases := []testCase{
		getThingSuccess,
		getThingSuccessWithRelationShip,
		getThingNotFound,
		getThingWithInvalidUUID,
		getThingWithConceptsAPIError,
		getThingWithConceptsAPIInvalidResponse,
		getThingRedirect,
		getThingRedirectWithRelationships,
		getThingsWithoutParams,
		getThingsNotFound,
		getThingsWithInvalidUUID,
		getThingsWithAlternativeUUID,
		getBrand,
	}
	for _, test := range testCases {
		mockClient.resp = test.clientBody
		mockClient.statusCode = test.clientCode
		mockClient.err = test.clientError

		handler := NewHandler(&mockClient, "localhost:8080/concepts")
		handler.RegisterHandlers(router)

		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", test.url, nil)

		router.ServeHTTP(rr, req)
		assert.Equal(t, test.expectedCode, rr.Code, test.name+" failed: status codes do not match!")
		if rr.Code == http.StatusOK {
			assert.Equal(t, transformBody(test.expectedBody), rr.Body.String(), test.name+" failed: status body does not match!")
			continue
		}
		assert.Equal(t, test.expectedBody, rr.Body.String(), test.name+" failed: status body does not match!")
	}
}

func TestInvalidConceptsAPIURL(t *testing.T) {
	logger.InitLogger("test service", "debug")
	mockClient := mockHTTPClient{
		resp:       "",
		statusCode: 200,
		err:        nil,
	}
	router := mux.NewRouter()
	handler := NewHandler(&mockClient, "://foo.com")
	handler.RegisterHandlers(router)
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/things/6773e864-78ab-4051-abc2-f4e9ab423ebb", nil)

	router.ServeHTTP(rr, req)
	assert.Equal(t, 503, rr.Code, "TestInvalidConceptsAPIURL failed: status codes do not match!")
	assert.Contains(t, rr.Body.String(), "Error getting thing with uuid 6773e864-78ab-4051-abc2-f4e9ab423ebb, err=parse", "TestInvalidConceptsAPIURL failed: status body does not match!")
}

func TestMethodNotAllowed(t *testing.T) {
	logger.InitLogger("test service", "debug")
	mockClient := mockHTTPClient{
		resp:       "",
		statusCode: 200,
		err:        nil,
	}
	router := mux.NewRouter()
	handler := NewHandler(&mockClient, "localhost:8080")
	handler.RegisterHandlers(router)
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/things/6773e864-78ab-4051-abc2-f4e9ab423ebb", nil)

	router.ServeHTTP(rr, req)
	assert.Equal(t, 405, rr.Code, "TestMethodNotAllowed failed: status codes do not match!")
}

func TestHealthCheck(t *testing.T) {
	logger.InitLogger("test service", "debug")

	mockClient := mockHTTPClient{
		statusCode: 200,
		err:        nil,
	}
	handler := NewHandler(&mockClient, "localhost:8080")

	healthCheck := fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  "public-things-api",
			Name:        "PublicThingsRead Healthcheck",
			Description: "Checks downstream services health",
			Checks:      []fthealth.Check{handler.HealthCheck()},
		},
		Timeout: 10 * time.Second,
	}
	router := mux.NewRouter()
	router.HandleFunc("/__health", fthealth.Handler(healthCheck))

	handler.RegisterHandlers(router)

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/__health", nil)

	logger.InitLogger("test service", "debug")

	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGtgCheck(t *testing.T) {
	logger.InitLogger("test service", "debug")

	mockClient := mockHTTPClient{
		statusCode: 200,
		err:        nil,
	}
	handler := NewHandler(&mockClient, "localhost:8080")

	healthCheck := fthealth.TimedHealthCheck{
		HealthCheck: fthealth.HealthCheck{
			SystemCode:  "public-things-api",
			Name:        "PublicThingsRead Healthcheck",
			Description: "Checks downstream services health",
			Checks:      []fthealth.Check{handler.HealthCheck()},
		},
		Timeout: 10 * time.Second,
	}
	router := mux.NewRouter()
	router.HandleFunc("/__GTG", fthealth.Handler(healthCheck))

	handler.RegisterHandlers(router)

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/__GTG", nil)

	logger.InitLogger("test service", "debug")

	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeprecatedConcept(t *testing.T) {
	logger.InitLogger("test service", "debug")
	var js BasicConcept

	mockClient := mockHTTPClient{
		resp:       deprecatedBrand,
		statusCode: 200,
		err:        nil,
	}
	router := mux.NewRouter()
	handler := NewHandler(&mockClient, "localhost:8080/concepts")

	handler.RegisterHandlers(router)

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/things/de5fe64c-eaf8-3232-8a0b-19f9ccdd5bf0", nil)

	router.ServeHTTP(rr, req)
	_ = json.Unmarshal([]byte(rr.Body.String()), &js)

	assert.Equal(t, true, js.IsDeprecated, "TestDeprecatedConcept failed: status body does not match!")
}

func TestSupersedesConcept(t *testing.T) {
	logger.InitLogger("test service", "debug")

	mockClient := mockHTTPClient{
		resp:       supersedesConcept,
		statusCode: 200,
		err:        nil,
	}
	router := mux.NewRouter()
	handler := NewHandler(&mockClient, "localhost:8080/concepts")

	handler.RegisterHandlers(router)

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/things/af42a8b7-a1f6-4c86-9df6-41e74266f541?showRelationship=related&showRelationship=broader", nil)

	router.ServeHTTP(rr, req)

	assert.Contains(t, rr.Body.String(), "http://www.ft.com/ontology/supersedes", "TestSuperseededConcept failed!")
}

var getCompleteThingAsConcept = `{
	"id": "http://www.ft.com/thing/6773e864-78ab-4051-abc2-f4e9ab423ebb",
	"apiUrl": "http://api.ft.com/concepts/6773e864-78ab-4051-abc2-f4e9ab423ebb",
	"type": "http://www.ft.com/ontology/product/Brand",
	"prefLabel": "Brussels blog",
	"descriptionXML": "This blog covers everything",
	"imageURL": "http://im.ft-static.com/content/images/2f1be452-02f3-11e6-99cb-83242733f755.png",
	"account": [
		{
			"type": "http://www.ft.com/ontology/twitterHandle",
			"value": "@ftbrussels"
		},
		{
			"type": "http://www.ft.com/ontology/facebookPage",
			"value": "https://www.facebook.com/financialtimes/"
		},
		{
			"type": "http://www.ft.com/ontology/emailAddress",
			"value": "example@example.com"
		}
	],
	"alternativeLabels": [
		{
			"type": "http://www.w3.org/2008/05/skos-xl#altLabel",
			"value": "Brussels Blog"
		}
	],
	"strapline": "Archived"
}`

var transformedCompleteThing = `{
	"id":"http://api.ft.com/things/6773e864-78ab-4051-abc2-f4e9ab423ebb",
	"apiUrl":"http://api.ft.com/brands/6773e864-78ab-4051-abc2-f4e9ab423ebb",
	"prefLabel":"Brussels blog",
	"types":[
		"http://www.ft.com/ontology/core/Thing",
		"http://www.ft.com/ontology/concept/Concept",
		"http://www.ft.com/ontology/classification/Classification",
		"http://www.ft.com/ontology/product/Brand"
	],
	"directType":"http://www.ft.com/ontology/product/Brand",
	"aliases":[
		"Brussels Blog"
	],
	"descriptionXML":"This blog covers everything",
	"_imageUrl":"http://im.ft-static.com/content/images/2f1be452-02f3-11e6-99cb-83242733f755.png",
	"emailAddress":"example@example.com",
	"facebookPage":"https://www.facebook.com/financialtimes/",
	"twitterHandle":"@ftbrussels"
}`

var getConmpleteThingWithRelationAsConcept = `{
	"id": "http://api.ft.com/things/6773e864-78ab-4051-abc2-f4e9ab423ebb",
	"apiUrl": "http://api.ft.com/brands/6773e864-78ab-4051-abc2-f4e9ab423ebb",
	"type": "http://www.ft.com/ontology/product/Brand",
	"prefLabel": "Brussels blog",
	"descriptionXML": "This blog covers everything",
	"imageURL": "http://im.ft-static.com/content/images/2f1be452-02f3-11e6-99cb-83242733f755.png",
	"account": [],
	"alternativeLabels": [],
	"relatedConcepts": [
		{
			"concept": {
				"id": "http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
				"apiUrl": "http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
				"type": "http://www.ft.com/ontology/Topic",
				"prefLabel": "Neglected tropical diseases",
				"alternativeLabels": []
			},
			"predicate": "http://www.ft.com/ontology/related"
		}
	],
	"broaderConcepts": [
		{
			"concept": {
				"id": "http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
				"apiUrl": "http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
				"type": "http://www.ft.com/ontology/Topic",
				"prefLabel": "Broader",
				"alternativeLabels": []
			},
			"predicate": "http://www.ft.com/ontology/broader"
		}
	],
	"narrowerConcepts": [
		{
			"concept": {
				"id": "http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
				"apiUrl": "http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
				"type": "http://www.ft.com/ontology/Topic",
				"prefLabel": "Narrower",
				"alternativeLabels": []
			},
			"predicate": "http://www.ft.com/ontology/narrower"
		}
	]
}`

var transformedCompleteThingWithRelation = `{
	"id":"http://api.ft.com/things/6773e864-78ab-4051-abc2-f4e9ab423ebb",
	"apiUrl":"http://api.ft.com/brands/6773e864-78ab-4051-abc2-f4e9ab423ebb",
	"prefLabel":"Brussels blog",
	"types":[
		"http://www.ft.com/ontology/core/Thing",
		"http://www.ft.com/ontology/concept/Concept",
		"http://www.ft.com/ontology/classification/Classification",
		"http://www.ft.com/ontology/product/Brand"
	],
	"directType":"http://www.ft.com/ontology/product/Brand",
	"descriptionXML":"This blog covers everything",
	"_imageUrl":"http://im.ft-static.com/content/images/2f1be452-02f3-11e6-99cb-83242733f755.png",
	"narrowerConcepts":[
		{
			"id":"http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
			"apiUrl":"http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
			"prefLabel":"Narrower",
			"types":[
				"http://www.ft.com/ontology/core/Thing",
				"http://www.ft.com/ontology/concept/Concept",
				"http://www.ft.com/ontology/Topic"
			],
			"directType":"http://www.ft.com/ontology/Topic",
			"predicate":"http://www.ft.com/ontology/narrower"
		}
	],
	"broaderConcepts":[
		{
			"id":"http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
			"apiUrl":"http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
			"prefLabel":"Broader",
			"types":[
				"http://www.ft.com/ontology/core/Thing",
				"http://www.ft.com/ontology/concept/Concept",
				"http://www.ft.com/ontology/Topic"
			],
			"directType":"http://www.ft.com/ontology/Topic",
			"predicate":"http://www.ft.com/ontology/broader"
		}
	],
	"relatedConcepts":[
		{
			"id":"http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
			"apiUrl":"http://api.ft.com/things/50d8fd9f-c4f3-42ae-9467-84a842c3c829",
			"prefLabel":"Neglected tropical diseases",
			"types":[
				"http://www.ft.com/ontology/core/Thing",
				"http://www.ft.com/ontology/concept/Concept",
				"http://www.ft.com/ontology/Topic"
			],
			"directType":"http://www.ft.com/ontology/Topic",
			"predicate":"http://www.ft.com/ontology/related"
		}
	]
}`

var brandAsConcept = `{
	"id":"http://api.ft.com/things/c3e3fe44-93fb-11e8-8f42-da24cd01f044",
	"apiUrl":"http://api.ft.com/concepts/c3e3fe44-93fb-11e8-8f42-da24cd01f044",
	"prefLabel":"Brussels Blog",
	"type":"http://www.ft.com/ontology/product/Brand",
	"alternativeLabels":[
		{
			"type": "http://www.w3.org/2008/05/skos-xl#altLabel",
			"value": "Brussels Blog"
		},
		{
			"type": "http://www.ft.com/ontology/shortLabel",
			"value": "BrusselsBlog"
		}
	],
	"broaderConcepts": [
		{
			
			"predicate": "http://www.ft.com/ontology/subBrandOf",
			"concept": {
				"id": "http://api.ft.com/things/58ff7494-8684-4473-a73d-1c02715be17e",
				"apiUrl": "http://api.ft.com/concepts/58ff7494-8684-4473-a73d-1c02715be17e",
				"type": "http://www.ft.com/ontology/product/Brand",
				"prefLabel": "Broader"
			}
		}
	],
	"narrowerConcepts": [
		{
			"predicate": "http://www.ft.com/ontology/hasSubBrand",
			"concept": {
				"id": "http://api.ft.com/things/1c4e60c4-93fc-11e8-8f42-da24cd01f044",
				"apiUrl": "http://api.ft.com/concepts/1c4e60c4-93fc-11e8-8f42-da24cd01f044",
				"type": "http://www.ft.com/ontology/product/Brand",
				"prefLabel": "Narrower"
			}
		}
	]
}`

var transformedBrand = `{
	"id":"http://api.ft.com/things/c3e3fe44-93fb-11e8-8f42-da24cd01f044",
	"apiUrl":"http://api.ft.com/brands/c3e3fe44-93fb-11e8-8f42-da24cd01f044",
	"prefLabel":"Brussels Blog",
	"types":[
		"http://www.ft.com/ontology/core/Thing",
		"http://www.ft.com/ontology/concept/Concept",
		"http://www.ft.com/ontology/classification/Classification",
		"http://www.ft.com/ontology/product/Brand"
	],
	"directType":"http://www.ft.com/ontology/product/Brand",
	"aliases":[
		"Brussels Blog"
	],
	"shortLabel":"BrusselsBlog",
	"narrowerConcepts":[
		{
			"id":"http://api.ft.com/things/1c4e60c4-93fc-11e8-8f42-da24cd01f044",
			"apiUrl":"http://api.ft.com/brands/1c4e60c4-93fc-11e8-8f42-da24cd01f044",
			"prefLabel":"Narrower",
			"types":[
				"http://www.ft.com/ontology/core/Thing",
				"http://www.ft.com/ontology/concept/Concept",
				"http://www.ft.com/ontology/classification/Classification",
				"http://www.ft.com/ontology/product/Brand"
			],
			"directType":"http://www.ft.com/ontology/product/Brand",
			"predicate":"http://www.w3.org/2004/02/skos/core#narrower"
		}
	],
	"broaderConcepts":[
		{
			"id":"http://api.ft.com/things/58ff7494-8684-4473-a73d-1c02715be17e",
			"apiUrl":"http://api.ft.com/brands/58ff7494-8684-4473-a73d-1c02715be17e",
			"prefLabel":"Broader",
			"types":[
				"http://www.ft.com/ontology/core/Thing",
				"http://www.ft.com/ontology/concept/Concept",
				"http://www.ft.com/ontology/classification/Classification",
				"http://www.ft.com/ontology/product/Brand"
			],
			"directType":"http://www.ft.com/ontology/product/Brand",
			"predicate":"http://www.w3.org/2004/02/skos/core#broader"
		}
	]
}`

var deprecatedBrand = `{
    "id": "http://api.ft.com/things/de5fe64c-eaf8-3232-8a0b-19f9ccdd5bf0",
    "apiUrl": "http://api.ft.com/brands/de5fe64c-eaf8-3232-8a0b-19f9ccdd5bf0",
	"prefLabel": "Professor of the Month",
	"isDeprecated": true,
	"types": [
        "http://www.ft.com/ontology/core/Thing",
        "http://www.ft.com/ontology/concept/Concept",
        "http://www.ft.com/ontology/classification/Classification",
        "http://www.ft.com/ontology/product/Brand"
    ],
    "directType": "http://www.ft.com/ontology/product/Brand",
    "aliases": [
        "Professor of the Month"
    ]
}`

var supersedesConcept = `{
    "id": "http://api.ft.com/things/af42a8b7-a1f6-4c86-9df6-41e74266f541",
    "apiUrl": "http://api.ft.com/things/af42a8b7-a1f6-4c86-9df6-41e74266f541",
    "prefLabel": "Agriculture",
    "types": [
        "http://www.ft.com/ontology/core/Thing",
        "http://www.ft.com/ontology/concept/Concept",
        "http://www.ft.com/ontology/Topic"
    ],
    "directType": "http://www.ft.com/ontology/Topic",
    "aliases": [
        "Agriculture"
    ],
    "relatedConcepts": [
        {
            "id": "http://api.ft.com/things/9d4881fa-f11b-4bb8-9746-2a8271b14a4f",
            "apiUrl": "http://api.ft.com/things/9d4881fa-f11b-4bb8-9746-2a8271b14a4f",
            "prefLabel": "Agricultural commodities",
            "types": [
                "http://www.ft.com/ontology/core/Thing",
                "http://www.ft.com/ontology/concept/Concept",
                "http://www.ft.com/ontology/Topic"
            ],
            "directType": "http://www.ft.com/ontology/Topic",
            "predicate": "http://www.w3.org/2004/02/skos/core#related"
        },
        {
            "id": "http://api.ft.com/things/414f547c-9768-4adc-ac6d-d7cf7f62b315",
            "apiUrl": "http://api.ft.com/things/414f547c-9768-4adc-ac6d-d7cf7f62b315",
            "prefLabel": "EU agriculture",
            "types": [
                "http://www.ft.com/ontology/core/Thing",
                "http://www.ft.com/ontology/concept/Concept",
                "http://www.ft.com/ontology/Topic"
            ],
            "directType": "http://www.ft.com/ontology/Topic",
            "predicate": "http://www.ft.com/ontology/supersedes",
            "isDeprecated": true
        }
    ]
}`

func transformBody(testBody string) string {
	stripNewLines := strings.Replace(testBody, "\n", "", -1)
	stripTabs := strings.Replace(stripNewLines, "\t", "", -1)
	return stripTabs + "\n"
}

type mockedDriver struct {
	mock.Mock
}

func (m *mockedDriver) read(id string, relationships []string) (thing Concept, found bool, err error) {
	args := m.Called(id, relationships)
	return args.Get(0).(Concept), args.Bool(1), args.Error(2)
}

func (m *mockedDriver) checkConnectivity() error {
	args := m.Called()
	return args.Error(0)
}
