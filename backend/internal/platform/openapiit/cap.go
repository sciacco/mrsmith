package openapiit

import (
	"context"
	"net/url"
	"strings"
)

type CAPClient struct {
	client *Client
}

type MunicipalitySearchParams struct {
	Comune        string
	CAP           string
	ISTAT         string
	CodiceCatasto string
	Regione       string
	Provincia     string
}

func (c *CAPClient) SearchMunicipalities(ctx context.Context, params MunicipalitySearchParams) (Envelope[MunicipalitySearchData], error) {
	var out Envelope[MunicipalitySearchData]
	err := c.client.getCAP(ctx, "/cerca_comuni", params.values(), &out)
	return out, err
}

func (c *CAPClient) GetMunicipalityBase(ctx context.Context, code string) (Envelope[MunicipalityBase], error) {
	var out Envelope[MunicipalityBase]
	err := c.client.getCAP(ctx, "/comuni_base/"+pathSegment(code), nil, &out)
	return out, err
}

func (c *CAPClient) GetMunicipalityAdvanced(ctx context.Context, code string) (Envelope[MunicipalityAdvanced], error) {
	var out Envelope[MunicipalityAdvanced]
	err := c.client.getCAP(ctx, "/comuni_advance/"+pathSegment(code), nil, &out)
	return out, err
}

func (c *CAPClient) LookupCAP(ctx context.Context, cap string) (Envelope[CAPLookup], error) {
	var out Envelope[CAPLookup]
	err := c.client.getCAP(ctx, "/cap/"+pathSegment(cap), nil, &out)
	return out, err
}

func (c *CAPClient) ListRegions(ctx context.Context) (Envelope[[]Region], error) {
	var out Envelope[[]Region]
	err := c.client.getCAP(ctx, "/regioni", nil, &out)
	return out, err
}

func (c *CAPClient) ListProvinces(ctx context.Context) (Envelope[[]Province], error) {
	var out Envelope[[]Province]
	err := c.client.getCAP(ctx, "/province", nil, &out)
	return out, err
}

func (c *CAPClient) GetProvince(ctx context.Context, code string) (Envelope[Province], error) {
	var out Envelope[Province]
	err := c.client.getCAP(ctx, "/province/"+pathSegment(code), nil, &out)
	return out, err
}

func (c *CAPClient) ListDUG(ctx context.Context) (Envelope[[]DUG], error) {
	var out Envelope[[]DUG]
	err := c.client.getCAP(ctx, "/dug", nil, &out)
	return out, err
}

func (c *CAPClient) ListSuppressedMunicipalities(ctx context.Context, provinceCode string) (Envelope[[]SuppressedMunicipality], error) {
	var out Envelope[[]SuppressedMunicipality]
	query := url.Values{}
	setQueryValue(query, "sigla_provincia", provinceCode)
	err := c.client.getCAP(ctx, "/comuni_soppressi", query, &out)
	return out, err
}

func (c *CAPClient) ListMetropolitanCities(ctx context.Context) (Envelope[[]MetropolitanCity], error) {
	var out Envelope[[]MetropolitanCity]
	err := c.client.getCAP(ctx, "/citta_metropolitane", nil, &out)
	return out, err
}

func (p MunicipalitySearchParams) values() url.Values {
	query := url.Values{}
	setQueryValue(query, "comune", p.Comune)
	setQueryValue(query, "cap", p.CAP)
	setQueryValue(query, "istat", p.ISTAT)
	setQueryValue(query, "codice_catasto", p.CodiceCatasto)
	setQueryValue(query, "regione", p.Regione)
	setQueryValue(query, "provincia", p.Provincia)
	return query
}

func setQueryValue(query url.Values, key string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		query.Set(key, value)
	}
}

func pathSegment(value string) string {
	return url.PathEscape(strings.TrimSpace(value))
}
