package openapiit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type StringValue string

func (s *StringValue) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*s = ""
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = StringValue(text)
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	switch typed := value.(type) {
	case json.Number:
		*s = StringValue(typed.String())
		return nil
	case bool:
		if typed {
			*s = "true"
		} else {
			*s = "false"
		}
		return nil
	default:
		return fmt.Errorf("unsupported string value %T", value)
	}
}

func (s StringValue) String() string {
	return string(s)
}

type MunicipalitySearchData struct {
	Result     []MunicipalitySearchResult      `json:"result"`
	Suppressed []SuppressedMunicipalitySummary `json:"suppressed"`
}

type MunicipalitySearchResult struct {
	ISTAT      string  `json:"istat"`
	Comune     string  `json:"comune"`
	Frazione   *string `json:"frazione"`
	Suppressed bool    `json:"suppressed"`
}

type SuppressedMunicipalitySummary struct {
	ISTAT      string `json:"istat"`
	Comune     string `json:"comune"`
	Suppressed bool   `json:"suppressed"`
}

type MunicipalityBase struct {
	ISTAT     string   `json:"istat"`
	Comune    string   `json:"comune"`
	Regione   string   `json:"regione"`
	Provincia string   `json:"provincia"`
	CAP       []string `json:"cap"`
}

type MunicipalityAdvanced struct {
	ISTAT          string              `json:"istat"`
	Comune         string              `json:"comune"`
	Regione        string              `json:"regione"`
	Provincia      string              `json:"provincia"`
	Prefisso       string              `json:"prefisso"`
	CodFisco       string              `json:"cod_fisco"`
	Superficie     int                 `json:"superficie"`
	NumResidenti   int                 `json:"num_residenti"`
	NomeAbitanti   string              `json:"nome_abitanti"`
	Patrono        *Patron             `json:"patrono"`
	Municipio      *MunicipalityOffice `json:"municipio"`
	ISTATOld       *string             `json:"istat_old"`
	SiglaProvincia string              `json:"sigla_provincia"`
	Email          string              `json:"email"`
	PEC            string              `json:"pec"`
	Tel            string              `json:"tel"`
	Fax            string              `json:"fax"`
	Frazioni       []string            `json:"frazioni"`
	CAP            []string            `json:"cap"`
	Sestieri       []Sestiere          `json:"sestieri"`
	Strade         StreetsByCAP        `json:"strade"`
}

type Patron struct {
	Nome string `json:"nome"`
	Data string `json:"data"`
}

type MunicipalityOffice struct {
	Municipio string `json:"municipio"`
}

type Sestiere struct {
	Quartiere string `json:"quartiere"`
	CAP       string `json:"cap"`
}

type StreetsByCAP map[string][]Street

type Street struct {
	DUG                     string              `json:"dug"`
	DUGComplemento          *string             `json:"dug_complemento"`
	Nome                    string              `json:"nome"`
	DUGApici                string              `json:"dug_apici"`
	DUGComplementoApici     *string             `json:"dug_complemento_apici"`
	NomeApici               string              `json:"nome_apici"`
	DenominazioneAbbreviata string              `json:"denominazione_abbreviata"`
	ArchiStradali           []StreetNumberRange `json:"archi_stradali"`
}

type StreetNumberRange struct {
	Dal    string  `json:"dal"`
	Al     string  `json:"al"`
	Parita string  `json:"parita"`
	Colore *string `json:"colore"`
}

type CAPLookup struct {
	Comuni         []CAPMunicipality `json:"comuni"`
	Regione        string            `json:"regione"`
	Provincia      string            `json:"provincia"`
	SiglaProvincia string            `json:"sigla_provincia"`
}

type CAPMunicipality struct {
	ISTAT       string       `json:"istat"`
	Comune      string       `json:"comune"`
	Frazione    *string      `json:"frazione"`
	ComuneISTAT string       `json:"comune_istat"`
	MultiCAP    bool         `json:"multi_cap"`
	Strade      StreetsByCAP `json:"strade"`
}

type Region struct {
	Regione      string  `json:"regione"`
	Capoluogo    string  `json:"capoluogo"`
	Superficie   float64 `json:"superficie"`
	NumResidenti int     `json:"num_residenti"`
	NumProvince  int     `json:"num_province"`
	NumComuni    int     `json:"num_comuni"`
	Presidente   string  `json:"presidente"`
	CodFiscale   string  `json:"cod_fiscale"`
	PIVA         string  `json:"piva"`
	PEC          string  `json:"pec"`
	Sito         string  `json:"sito"`
	Sede         string  `json:"sede"`
	ISTAT        string  `json:"istat"`
}

type Province struct {
	Sigla      string  `json:"sigla"`
	Provincia  string  `json:"provincia"`
	Superficie float64 `json:"superficie"`
	Residenti  int     `json:"residenti"`
	NumComuni  int     `json:"num_comuni"`
	ISTAT      string  `json:"istat"`
	Regione    string  `json:"regione"`
}

type DUG struct {
	DUG           string `json:"dug"`
	DUGAbbreviata string `json:"dug_abbreviata"`
}

type SuppressedMunicipality struct {
	ISTAT          string      `json:"istat"`
	Comune         string      `json:"comune"`
	CodFisco       StringValue `json:"cod_fisco"`
	SiglaProvincia string      `json:"sigla_provincia"`
	Regione        string      `json:"regione"`
	Provincia      string      `json:"provincia"`
}

type MetropolitanCity struct {
	Denominazione string `json:"denominazione"`
	Capoluogo     string `json:"capoluogo"`
	Popolazione   int    `json:"popolazione"`
	Superficie    int    `json:"superficie"`
	Densita       int    `json:"densita"`
	NumComuni     int    `json:"num_comuni"`
}
