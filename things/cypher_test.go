package things

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"reflect"
	"sort"

	"github.com/Financial-Times/base-ft-rw-app-go/baseftrwapp"
	"github.com/Financial-Times/concepts-rw-neo4j/concepts"
	"github.com/Financial-Times/content-rw-neo4j/content"
	"github.com/Financial-Times/neo-model-utils-go/mapper"
	"github.com/Financial-Times/neo-utils-go/neoutils"
	"github.com/Financial-Times/organisations-rw-neo4j/organisations"
	"github.com/Financial-Times/people-rw-neo4j/people"
	"github.com/jmcvetta/neoism"
	_ "github.com/joho/godotenv/autoload"
	"github.com/stretchr/testify/assert"
)

const (
	//Generate uuids so there's no clash with real data
	FakebookConceptUUID    = "eac853f5-3859-4c08-8540-55e043719400" //organisation - Old concept model
	MetalMickeyConceptUUID = "0483bef8-5797-40b8-9b25-b12e492f63c6" //subject - ie. New concept model
	ContentUUID            = "3fc9fe3e-af8c-4f7f-961a-e5065392bb31"
	NonExistingThingUUID   = "b2860919-4b78-44c6-a665-af9221bdefb5"
	PersonThingUUID        = "75e2f7e9-cb5e-40a5-a074-86d69fe09f69"
	TopicOnyxPike          = "9a07c16f-def0-457d-a04a-57ba68ba1e00"
	TopicOnyxPikeRelated   = "ec20c787-8289-4cef-aee8-4d39e9563dc5"
	TopicOnyxPikeBroader   = "ba42b8d0-844f-4f2a-856c-5cbd863bf6bd"
)

//Reusable Neo4J connection
var db neoutils.NeoConnection

func init() {
	// We are initialising a lot of constraints on an empty database therefore we need the database to be fit before
	// we run tests so initialising the service will create the constraints first
	conf := neoutils.DefaultConnectionConfig()
	conf.Transactional = false
	db, _ = neoutils.Connect(neoUrl(), conf)
	if db == nil {
		panic("Cannot connect to Neo4J")
	}

}

func neoUrl() string {
	url := os.Getenv("NEO4J_TEST_URL")
	if url == "" {
		url = "http://localhost:7474/db/data"
	}
	return url
}

func TestRetrievePeopleAsThing(t *testing.T) {
	defer cleanDB(t, PersonThingUUID)

	personRW := people.NewCypherPeopleService(db)
	assert.NoError(t, personRW.Initialise())

	writeJSONToService(t, personRW, fmt.Sprintf("./fixtures/People-%s.json", PersonThingUUID))
	types := []string{"Thing", "Concept", "Person"}
	typesUris := mapper.TypeURIs(types)

	thingsDriver := NewCypherDriver(db, "prod")
	thng, found, err := thingsDriver.read(PersonThingUUID)
	assert.NoError(t, err, "Unexpected error for person as thing %s", PersonThingUUID)
	assert.True(t, found, "Found no Concept for person as Concept %s", PersonThingUUID)

	expected := Concept{
		APIURL:         mapper.APIURL(PersonThingUUID, types, "Prod"),
		PrefLabel:      "John Smith",
		ID:             mapper.IDURL(PersonThingUUID),
		Types:          typesUris,
		DirectType:     typesUris[len(typesUris)-1],
		Aliases:        []string{"John Smith"},
		EmailAddress:   "john.smith@ft.com",
		TwitterHandle:  "@johnsmith",
		DescriptionXML: "John smith is some bloke and a beer",
	}

	readAndCompare(t, expected, thng, "Person successful retrieval")
}

func TestRetrieveOrganisationAsThing(t *testing.T) {
	defer cleanDB(t, FakebookConceptUUID)

	organisationRW := organisations.NewCypherOrganisationService(db)
	assert.NoError(t, organisationRW.Initialise())

	types := []string{"Thing", "Concept", "Organisation", "Company", "PublicCompany"}
	typesUris := mapper.TypeURIs(types)

	expected := Concept{APIURL: mapper.APIURL(FakebookConceptUUID, types, "Prod"), PrefLabel: "Fakebook, Inc.", ID: mapper.IDURL(FakebookConceptUUID),
		Types: typesUris, DirectType: typesUris[len(typesUris)-1]}

	writeJSONToService(t, organisationRW, fmt.Sprintf("./fixtures/Organisation-Fakebook-%v.json", FakebookConceptUUID))

	thingsDriver := NewCypherDriver(db, "prod")
	thng, found, err := thingsDriver.read(FakebookConceptUUID)
	assert.NoError(t, err, "Unexpected error for organisation as thing %s", FakebookConceptUUID)
	assert.True(t, found, "Found no Concept for organisation as Concept %s", FakebookConceptUUID)

	readAndCompare(t, expected, thng, "Organisation successful retrieval")
}

func TestRetrieveConceptNewModelAsThing(t *testing.T) {
	defer cleanDB(t, TopicOnyxPikeRelated, TopicOnyxPikeBroader, TopicOnyxPike)

	conceptsDriver := concepts.NewConceptService(db)
	assert.NoError(t, conceptsDriver.Initialise())

	types := []string{"Thing", "Concept", "Topic"}
	typesUris := mapper.TypeURIs(types)

	expected := Concept{APIURL: mapper.APIURL(TopicOnyxPike, types, "Prod"), PrefLabel: "Onyx Pike", ID: mapper.IDURL(TopicOnyxPike),
		Types: typesUris, DirectType: typesUris[len(typesUris)-1], Aliases: []string{"Bob", "BOB2"},
		DescriptionXML: "<p>Some stuff</p>", ImageURL: "http://media.ft.com/brand.png", EmailAddress:"email@email.com", ScopeNote: "bobs scopey notey", ShortLabel: "Short Label", TwitterHandle: "bob@twitter.com", FacebookPage: "bob@facebook.com",
		BroaderThan: []Thing{{ID: mapper.IDURL(TopicOnyxPikeBroader), APIURL: mapper.APIURL(TopicOnyxPikeBroader, types, "Prod"), Types: typesUris, PrefLabel: "Onyx Pike Broader", DirectType: typesUris[len(typesUris)-1]}},
		RelatedTo:[]Thing{{ID: mapper.IDURL(TopicOnyxPikeRelated), APIURL: mapper.APIURL(TopicOnyxPikeRelated, types, "Prod"),  Types: typesUris, PrefLabel: "Onyx Pike Related", DirectType: typesUris[len(typesUris)-1]}}}

	writeJSONToService(t, conceptsDriver, fmt.Sprintf("./fixtures/Topic-OnyxPikeRelated-%s.json", TopicOnyxPikeRelated))
	writeJSONToService(t, conceptsDriver, fmt.Sprintf("./fixtures/Topic-OnyxPikeBroader-%s.json",TopicOnyxPikeBroader))
	writeJSONToService(t, conceptsDriver, fmt.Sprintf("./fixtures/Topic-OnyxPike-%s.json", TopicOnyxPike))

	thingsDriver := NewCypherDriver(db, "prod")

	thng, found, err := thingsDriver.read(TopicOnyxPike)



	assert.NoError(t, err, "Unexpected error for concept as thing %s", TopicOnyxPike)
	assert.True(t, found, "Found no Concept for concept as Concept %s", TopicOnyxPike)

	readAndCompare(t, expected, thng, "Retrieve concepts via new concordance model")
}

func readAndCompare(t *testing.T, expected Concept, actual Concept, testName string) {
	sort.Slice(expected.Aliases, func(i, j int) bool {
		return expected.Aliases[i] < expected.Aliases[j]
	})

	sort.Slice(actual.Aliases, func(i, j int) bool {
		return actual.Aliases[i] < actual.Aliases[j]
	})

	sort.Slice(expected.Types, func(i, j int) bool {
		return expected.Types[i] < expected.Types[j]
	})

	sort.Slice(actual.Types, func(i, j int) bool {
		return actual.Types[i] < actual.Types[j]
	})

	sort.Slice(actual.BroaderThan, func(i, j int) bool {
		return actual.BroaderThan[i].ID < actual.BroaderThan[j].ID
	})

	sort.Slice(actual.NarrowerThan, func(i, j int) bool {
		return actual.NarrowerThan[i].ID< actual.NarrowerThan[j].ID
	})

	sort.Slice(actual.RelatedTo, func(i, j int) bool {
		return actual.RelatedTo[i].ID < actual.RelatedTo[j].ID
	})

	sort.Slice(expected.BroaderThan, func(i, j int) bool {
		return expected.BroaderThan[i].ID < expected.BroaderThan[j].ID
	})

	sort.Slice(expected.NarrowerThan, func(i, j int) bool {
		return expected.NarrowerThan[i].ID < expected.NarrowerThan[j].ID
	})

	sort.Slice(expected.RelatedTo, func(i, j int) bool {
		return expected.RelatedTo[i].ID < expected.RelatedTo[j].ID
	})

	for _, thing := range actual.NarrowerThan {
		sort.Slice(thing.Types, func(i, j int) bool {
			return thing.Types[i] < thing.Types[j]
		})
	}
	for _, thing := range actual.BroaderThan {
		sort.Slice(thing.Types, func(i, j int) bool {
			return thing.Types[i] < thing.Types[j]
		})
	}
	for _, thing := range actual.RelatedTo {
		sort.Slice(thing.Types, func(i, j int) bool {
			return thing.Types[i] < thing.Types[j]
		})
	}

	for _, thing := range expected.NarrowerThan {
		sort.Slice(thing.Types, func(i, j int) bool {
			return thing.Types[i] < thing.Types[j]
		})
	}
	for _, thing := range expected.BroaderThan {
		sort.Slice(thing.Types, func(i, j int) bool {
			return thing.Types[i] < thing.Types[j]
		})
	}
	for _, thing := range expected.RelatedTo {
		sort.Slice(thing.Types, func(i, j int) bool {
			return thing.Types[i] < thing.Types[j]
		})
	}

	assert.True(t, reflect.DeepEqual(expected, actual), fmt.Sprintf("Actual concept differs from expected: \n ExpectedConcept: %v \n Actual: %v", expected, actual))
}

//TODO - this is temporary, we WILL want to retrieve Content once we have more info about it available
func TestCannotRetrieveContentAsThing(t *testing.T) {
	assert := assert.New(t)

	contentRW := content.NewCypherContentService(db)
	assert.NoError(contentRW.Initialise())
	writeJSONToService(t, contentRW, "./fixtures/Content-Bitcoin-3fc9fe3e-af8c-4f7f-961a-e5065392bb31.json")

	defer cleanDB(t, NonExistingThingUUID, ContentUUID)

	thingsDriver := NewCypherDriver(db, "prod")
	thng, found, err := thingsDriver.read(NonExistingThingUUID)
	assert.NoError(err, "Unexpected error for thing %s", NonExistingThingUUID)
	assert.False(found, "Found thing %s", NonExistingThingUUID)
	assert.EqualValues(Concept{}, thng, "Found non-existing thing %s", NonExistingThingUUID)
}

func TestRetrieveNoThingsWhenThereAreNonePresent(t *testing.T) {
	assert := assert.New(t)
	thingsDriver := NewCypherDriver(db, "prod")
	thng, found, err := thingsDriver.read(NonExistingThingUUID)
	assert.NoError(err, "Unexpected error for thing %s", NonExistingThingUUID)
	assert.False(found, "Found thing %s", NonExistingThingUUID)
	assert.EqualValues(Concept{}, thng, "Found non-existing thing %s", NonExistingThingUUID)
}

func cleanDB(t *testing.T, uuids ...string) {
	qs := make([]*neoism.CypherQuery, len(uuids))
	for i, uuid := range uuids {
		qs[i] = &neoism.CypherQuery{
			Statement: fmt.Sprintf(`
			MATCH (a:Thing {uuid: "%s"})
			OPTIONAL MATCH (a)-[ids:IDENTIFIES]-(c)
			OPTIONAL MATCH (related)-[rel]-(d)
			DELETE ids, rel
			DETACH DELETE c, d, a`, uuid)}
	}
	err := db.CypherBatch(qs)
	assert.NoError(t, err, "Error executing clean up cypher")
}

func writeJSONToService(t *testing.T, service baseftrwapp.Service, pathToJSONFile string) {
	f, err := os.Open(pathToJSONFile)
	assert.NoError(t, err)
	dec := json.NewDecoder(f)
	inst, _, errr := service.DecodeJSON(dec)
	assert.NoError(t, errr)

	errs := service.Write(inst, "TRANS_ID")
	assert.NoError(t, errs)
}

func validateThing(t *testing.T, prefLabel string, UUID string, directType string, types []string, thng Concept) {
	assert.EqualValues(t, prefLabel, thng.PrefLabel, "PrefLabel incorrect")
	assert.EqualValues(t, "http://api.ft.com/Things/"+UUID, thng.ID, "ID incorrect")
	assert.EqualValues(t, directType, thng.DirectType, "DirectType incorrect")
	assert.EqualValues(t, types, thng.Types, "Types incorrect")
}
