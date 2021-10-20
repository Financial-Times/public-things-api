package things

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"io/ioutil"

	fthealth "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/neo-model-utils-go/mapper"
	"github.com/Financial-Times/service-status-go/gtg"
	"github.com/Financial-Times/transactionid-utils-go"
	"github.com/gorilla/mux"

	gouuid "github.com/google/uuid"
)

var CacheControlHeader string

const (
	validUUID       = "([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})$"
	shortLabelURI   = "http://www.ft.com/ontology/shortLabel"
	aliasLabelURI   = "http://www.w3.org/2008/05/skos-xl#altLabel"
	emailAddressURI = "http://www.ft.com/ontology/emailAddress"
	facebookPageURI = "http://www.ft.com/ontology/facebookPage"
	twitterURI      = "http://www.ft.com/ontology/twitterHandle"
	thingsApiUrl    = "http://api.ft.com/things/"
	ftThing         = "http://www.ft.com/thing/"
)

var brandPredicateMap = map[string]string{
	"http://www.ft.com/ontology/subBrandOf":  "http://www.w3.org/2004/02/skos/core#broader",
	"http://www.ft.com/ontology/hasSubBrand": "http://www.w3.org/2004/02/skos/core#narrower",
}

type HttpClient interface {
	Do(req *http.Request) (resp *http.Response, err error)
}

type ThingsHandler struct {
	client      HttpClient
	conceptsURL string
}

func NewHandler(client HttpClient, conceptsURL string) ThingsHandler {
	return ThingsHandler{
		client,
		conceptsURL,
	}
}

func (h *ThingsHandler) RegisterHandlers(router *mux.Router) {
	logger.Info("Registering handlers")
	router.HandleFunc("/things/{uuid}", h.GetThing).Methods("GET")
	router.HandleFunc("/things", h.GetThings).Methods("GET")
}

func (h *ThingsHandler) HealthCheck() fthealth.Check {
	return fthealth.Check{
		ID:               "public-concepts-api-check",
		BusinessImpact:   "Unable to respond to Public Things api requests",
		Name:             "Check connectivity to public-concepts-api",
		PanicGuide:       "https://dewey.ft.com/public-things-api.html",
		Severity:         2,
		TechnicalSummary: "Not being able to communicate with public-concepts-api means that requests for things cannot be performed.",
		Checker:          h.Checker,
	}
}

// GetThing handler directly returns the concept/thing if it's a canonical
// or provides redirect URL via Location http header within the response.
func (rh *ThingsHandler) GetThing(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	uuid := vars["uuid"]
	transID := transactionidutils.GetTransactionIDFromRequest(r)
	relationships := r.URL.Query()["showRelationship"]
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	if uuid == "" {
		http.Error(w, "uuid required", http.StatusBadRequest)
		return
	}

	if err := validateUUID(uuid); err != nil {
		http.Error(w, "invalid/malformed uuid", http.StatusBadRequest)
		return
	}

	thing, found, err := rh.getThingViaConceptsApi(uuid, relationships, transID)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		msg := fmt.Sprintf(`{"message":"Error getting thing with uuid %s, err=%s"}`, uuid, err.Error())
		w.Write([]byte(msg))
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		msg := fmt.Sprintf(`{"message":"No thing found with uuid %s."}`, uuid)
		w.Write([]byte(msg))
		return
	}

	//if the request was not made for the canonical, but an alternate uuid: redirect
	if !strings.Contains(thing.ID, uuid) {
		validRegexp := regexp.MustCompile(validUUID)
		canonicalUUID := validRegexp.FindString(thing.ID)
		redirectURL := strings.Replace(r.URL.String(), uuid, canonicalUUID, 1)
		w.Header().Set("Location", redirectURL)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	w.Header().Set("Cache-Control", CacheControlHeader)
	w.WriteHeader(http.StatusOK)

	if err = json.NewEncoder(w).Encode(thing); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf(`{"message":"Error parsing thing with uuid %s, err=%s"}`, uuid, err.Error())
		w.Write([]byte(msg))
	}
}

// GetThings handler provides a batch like functionality, quite similar to single get endpoint.
// Implementation schedules a new go routine for every requested "thing" uuid, wait for the results and returns
// the aggregated results to caller.
//
// Non canonical uuid handling:
//
// 	Implementation slightly deviates from the single get endpoint for non canonical uuids.
// 	It tries to resolve the canonical uuid/node itself instead of providing a reference url but strictly stops
// 	if indirection dept is more than one level.
//
// Error handling:
//
// 	In case of any error for any given uuid, implementation immediately returns with the error without waiting for the
// 	in-flight queries to be finished.
//
// Response structure:
//
// 	Instead of returning an array of things, we are returning/serializing a map of ["things":{[uuid:Thing]}]. Reason behind this
// 	is simply to provide a convenient way to the caller for making the correlation between requested uuids with respect to
// 	found things. Since we are handling the resolution of non canonical uuids, returned thing payloads may not have the same
// 	requested/associated uuid.
func (rh *ThingsHandler) GetThings(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	transID := transactionidutils.GetTransactionIDFromRequest(r)
	relationships := queryParams["showRelationship"]
	uuids := queryParams["uuid"]

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	if len(uuids) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"at least one uuid query param should be provided for batch operations"}`))
		return
	}

	if err := validateUUID(uuids...); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf(`{"message":"%v"}`, err)))
		return
	}

	var wg sync.WaitGroup
	uctCh := make(chan *uuidConceptTuple)
	errCh := make(chan *uuidErrorTuple)

	// fill up the sync bucket
	wg.Add(len(uuids))

	// start getting things
	for _, uuid := range uuids {
		go rh.getChanneledThing(uuid, relationships, transID, uctCh, errCh, &wg)
	}

	// start watching the sync bucket and close the channel
	go closeOnDone(uctCh, &wg)

	// synchronize/wait for the results
	things, err := aggregateChanneledThings(uctCh, errCh)

	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		msg := fmt.Sprintf(`{"message":"Error getting thing with uuid %s, err=%s"}`, err.uuid, err.err.Error())
		w.Write([]byte(msg))
		return
	}

	w.Header().Set("Cache-Control", CacheControlHeader)
	w.WriteHeader(http.StatusOK)

	result := make(map[string]map[string]Concept)
	result["things"] = things

	if err := json.NewEncoder(w).Encode(result); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf(`{"message":"Error marshalling the result %s, err=%s"}`, w, err.Error())
		w.Write([]byte(msg))
	}
}

func (rh *ThingsHandler) getChanneledThing(uuid string, relationships []string, transID string, uctCh chan *uuidConceptTuple,
	errCh chan *uuidErrorTuple, wg *sync.WaitGroup) {

	defer wg.Done()
	thing, found, err := rh.getThingViaConceptsApi(uuid, relationships, transID)

	if err != nil {
		errCh <- &uuidErrorTuple{uuid, err}
		return
	}

	if !found {
		return
	}

	if !strings.Contains(thing.ID, uuid) {
		validRegexp := regexp.MustCompile(validUUID)

		canonicalUUID := validRegexp.FindString(thing.ID)
		thing, found, err = rh.getThingViaConceptsApi(canonicalUUID, relationships, transID)

		if err != nil {
			errCh <- &uuidErrorTuple{uuid, err}
			return
		}

		if !found {
			logger.Errorf("Referenced canonical uuid : %s is missing in graph store for %s, possible data inconsistency",
				canonicalUUID, uuid)
			return
		}

		if !strings.Contains(thing.ID, canonicalUUID) {
			// there should be one level of indirection to the canonical node
			logger.Warnf("Multiple level of indirection to canonical node for uuid: %s, giving up traversing", uuid)
			return
		}
	}

	uctCh <- &uuidConceptTuple{uuid, thing}
}

func aggregateChanneledThings(uctCh chan *uuidConceptTuple, errCh chan *uuidErrorTuple) (map[string]Concept, *uuidErrorTuple) {
	things := make(map[string]Concept)

infiniteUntilClosed:
	for {
		select {
		case tuple, open := <-uctCh:
			if !open {
				break infiniteUntilClosed
			}
			things[tuple.uuid] = tuple.concept
		case err := <-errCh:
			return nil, err
		}
	}
	return things, nil
}

func closeOnDone(uctCh chan *uuidConceptTuple, wg *sync.WaitGroup) {
	wg.Wait()
	close(uctCh)
}

type uuidConceptTuple struct {
	uuid    string
	concept Concept
}

type uuidErrorTuple struct {
	uuid string
	err  error
}

func validateUUID(uuids ...string) error {
	for _, uuid := range uuids {
		_, err := gouuid.Parse(uuid)
		if err != nil {
			return errors.New(fmt.Sprintf("Invalid uuid: %s, err: %v", uuid, err))
		}
	}
	return nil
}

func (rh *ThingsHandler) getThingViaConceptsApi(UUID string, relationships []string, transID string) (Concept, bool, error) {
	mappedConcept := Concept{}

	u, err := url.Parse(rh.conceptsURL)
	if err != nil {
		msg := fmt.Sprint("URL of Concepts API is invalid")
		logger.WithError(err).WithUUID(UUID).WithTransactionID(transID).Error(msg)
		return mappedConcept, false, err
	}
	u.Path = "/concepts/" + UUID
	q := u.Query()
	for _, relationship := range relationships {
		q.Add("showRelationship", relationship)
	}
	u.RawQuery = q.Encode()
	reqURL := u.String()
	request, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		msg := fmt.Sprintf("failed to create request to %s", reqURL)
		logger.WithError(err).WithUUID(UUID).WithTransactionID(transID).Error(msg)
		return mappedConcept, false, err
	}

	request.Header.Set("X-Request-Id", transID)

	resp, err := rh.client.Do(request)
	if err != nil {
		msg := fmt.Sprintf("request to %s was unsuccessful", reqURL)
		logger.WithError(err).WithUUID(UUID).WithTransactionID(transID).Error(msg)
		return mappedConcept, false, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return mappedConcept, false, nil
	}

	conceptsApiResponse := ConceptApiResponse{}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		msg := fmt.Sprintf("failed to read response body: %v", resp.Body)
		logger.WithError(err).WithUUID(UUID).WithTransactionID(transID).Error(msg)
		return mappedConcept, false, err
	}
	if err = json.Unmarshal(body, &conceptsApiResponse); err != nil {
		msg := fmt.Sprintf("failed to unmarshal response body: %v", body)
		logger.WithError(err).WithUUID(UUID).WithTransactionID(transID).Error(msg)
		return mappedConcept, false, err
	}
	var altLabels []string
	mappedConcept.ID = convertID(conceptsApiResponse.ID)
	mappedConcept.APIURL = mapper.APIURL(UUID, []string{extractFinalSectionOfString(conceptsApiResponse.Type)}, "")
	mappedConcept.PrefLabel = conceptsApiResponse.PrefLabel
	mappedConcept.IsDeprecated = conceptsApiResponse.IsDeprecated
	mappedConcept.DirectType = conceptsApiResponse.Type
	mappedConcept.Types = mapper.FullTypeHierarchy(conceptsApiResponse.Type)

	for _, keypair := range conceptsApiResponse.AlternativeLabels {
		switch {
		case keypair.Type == aliasLabelURI:
			altLabels = append(altLabels, keypair.Value)
		case keypair.Type == shortLabelURI:
			mappedConcept.ShortLabel = keypair.Value
		}
	}
	mappedConcept.Aliases = altLabels
	mappedConcept.DescriptionXML = conceptsApiResponse.DescriptionXML
	mappedConcept.ImageURL = conceptsApiResponse.ImageURL
	for _, social := range conceptsApiResponse.Account {
		mapTypedValues(&mappedConcept, social)
	}
	mappedConcept.ScopeNote = conceptsApiResponse.ScopeNote

	if len(conceptsApiResponse.Broader) > 0 {
		mappedConcept.BroaderConcepts = convertRelationship(conceptsApiResponse.Broader)
	}
	if len(conceptsApiResponse.Narrower) > 0 {
		mappedConcept.NarrowerConcepts = convertRelationship(conceptsApiResponse.Narrower)
	}
	if len(conceptsApiResponse.Related) > 0 {
		mappedConcept.RelatedConcepts = convertRelationship(conceptsApiResponse.Related)
	}

	return mappedConcept, true, nil
}

func extractFinalSectionOfString(stringToTransform string) string {
	ss := strings.Split(stringToTransform, "/")
	return ss[len(ss)-1]
}

func convertRelationship(relationships []Relationship) []Thing {
	var convertedRelationships []Thing
	for _, rc := range relationships {
		convertedRelationships = append(convertedRelationships, Thing{
			ID:           convertID(rc.Concept.ID),
			APIURL:       mapper.APIURL(extractFinalSectionOfString(rc.Concept.ID), []string{extractFinalSectionOfString(rc.Concept.Type)}, ""),
			Types:        mapper.FullTypeHierarchy(rc.Concept.Type),
			DirectType:   rc.Concept.Type,
			PrefLabel:    rc.Concept.PrefLabel,
			IsDeprecated: rc.Concept.IsDeprecated,
			Predicate:    mapPredicate(rc.Predicate),
		})
	}
	return convertedRelationships
}

func mapTypedValues(concept *Concept, keypair TypedValue) {
	switch keypair.Type {
	case emailAddressURI:
		concept.EmailAddress = keypair.Value
	case facebookPageURI:
		concept.FacebookPage = keypair.Value
	case twitterURI:
		concept.TwitterHandle = keypair.Value
	default:
		logger.Errorf("Type %s not currently supported", keypair.Type)
	}
}

func convertID(conceptsApiID string) string {
	return strings.Replace(conceptsApiID, ftThing, thingsApiUrl, 1)
}

func mapPredicate(conceptPredicate string) string {
	if _, ok := brandPredicateMap[conceptPredicate]; ok {
		return brandPredicateMap[conceptPredicate]
	}
	return conceptPredicate
}

func (h *ThingsHandler) Checker() (string, error) {
	req, err := http.NewRequest("GET", h.conceptsURL+"/__gtg", nil)
	if err != nil {
		return "", err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("health check returned a non-200 HTTP status: %v", resp.StatusCode)
	}
	return "Public Concepts API is healthy", nil

}

func (h *ThingsHandler) GTG() gtg.Status {
	statusCheck := func() gtg.Status {
		return gtgCheck(h.Checker)
	}
	return gtg.FailFastParallelCheck([]gtg.StatusChecker{statusCheck})()
}

func gtgCheck(handler func() (string, error)) gtg.Status {
	if _, err := handler(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}
	return gtg.Status{GoodToGo: true}
}
