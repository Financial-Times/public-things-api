package things

type Things []Concept

type Concept struct {
	ID               string   `json:"id"`
	APIURL           string   `json:"apiUrl"`
	PrefLabel        string   `json:"prefLabel,omitempty"`
	Types            []string `json:"types"`
	DirectType       string   `json:"directType,omitempty"`
	Aliases          []string `json:"aliases,omitempty"`
	DescriptionXML   string   `json:"descriptionXML,omitempty"`
	ImageURL         string   `json:"_imageUrl,omitempty"`
	EmailAddress     string   `json:"emailAddress,omitempty"`
	FacebookPage     string   `json:"facebookPage,omitempty"`
	TwitterHandle    string   `json:"twitterHandle,omitempty"`
	ScopeNote        string   `json:"scopeNote,omitempty"`
	ShortLabel       string   `json:"shortLabel,omitempty"`
	NarrowerConcepts []Thing  `json:"narrowerConcepts,omitempty"`
	BroaderConcepts  []Thing  `json:"broaderConcepts,omitempty"`
	RelatedConcepts  []Thing  `json:"relatedConcepts,omitempty"`
	IsDeprecated     bool     `json:"isDeprecated,omitempty"`
}

type Thing struct {
	ID           string   `json:"id"`
	APIURL       string   `json:"apiUrl"`
	PrefLabel    string   `json:"prefLabel,omitempty"`
	Types        []string `json:"types"`
	DirectType   string   `json:"directType,omitempty"`
	Predicate    string   `json:"predicate,omitempty"`
	IsDeprecated bool     `json:"isDeprecated,omitempty"`
}

type ConceptApiResponse struct {
	BasicConcept
	DescriptionXML    string         `json:"descriptionXML,omitempty"`
	ImageURL          string         `json:"imageUrl,omitempty"`
	Account           []TypedValue   `json:"account,omitempty"`
	AlternativeLabels []TypedValue   `json:"alternativeLabels,omitempty"`
	ScopeNote         string         `json:"scopeNote,omitempty"`
	ShortLabel        string         `json:"shortLabel,omitempty"`
	Broader           []Relationship `json:"broaderConcepts,omitempty"`
	Narrower          []Relationship `json:"narrowerConcepts,omitempty"`
	Related           []Relationship `json:"relatedConcepts,omitempty"`
	IsDeprecated      bool           `json:"isDeprecated,omitempty"`
}

type TypedValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Relationship struct {
	Concept   BasicConcept `json:"concept,omitempty"`
	Predicate string       `json:"predicate,omitempty"`
}

type BasicConcept struct {
	ID           string `json:"id,omitempty"`
	ApiURL       string `json:"apiUrl,omitempty"`
	Type         string `json:"type,omitempty"`
	PrefLabel    string `json:"prefLabel,omitempty"`
	IsDeprecated bool   `json:"isDeprecated,omitempty"`
}
