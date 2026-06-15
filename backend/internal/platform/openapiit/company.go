package openapiit

import (
	"context"
	"net/url"
	"strconv"
)

type CompanyClient struct {
	client *Client
}

func (c *CompanyClient) GetITAML(ctx context.Context, vatCodeOrTaxCode string) (Envelope[CompanyDataset], error) {
	return c.getDataset(ctx, "/IT-aml/"+pathSegment(vatCodeOrTaxCode), nil)
}

func (c *CompanyClient) CreateITAMLRequest(ctx context.Context, vatCodeOrTaxCode string, body CompanyPostBody) (Envelope[CompanyRequest], error) {
	return c.postRequest(ctx, "/IT-aml/"+pathSegment(vatCodeOrTaxCode), body)
}

func (c *CompanyClient) GetITMarketing(ctx context.Context, vatCodeOrTaxCode string) (Envelope[CompanyDataset], error) {
	return c.getDataset(ctx, "/IT-marketing/"+pathSegment(vatCodeOrTaxCode), nil)
}

func (c *CompanyClient) CreateITMarketingRequest(ctx context.Context, vatCodeOrTaxCode string, body CompanyPostBody) (Envelope[CompanyRequest], error) {
	return c.postRequest(ctx, "/IT-marketing/"+pathSegment(vatCodeOrTaxCode), body)
}

func (c *CompanyClient) GetITStakeholders(ctx context.Context, vatCodeOrTaxCode string) (Envelope[CompanyDataset], error) {
	return c.getDataset(ctx, "/IT-stakeholders/"+pathSegment(vatCodeOrTaxCode), nil)
}

func (c *CompanyClient) CreateITStakeholdersRequest(ctx context.Context, vatCodeOrTaxCode string, body CompanyPostBody) (Envelope[CompanyRequest], error) {
	return c.postRequest(ctx, "/IT-stakeholders/"+pathSegment(vatCodeOrTaxCode), body)
}

func (c *CompanyClient) GetITFull(ctx context.Context, vatCodeOrTaxCode string) (Envelope[CompanyDataset], error) {
	return c.getDataset(ctx, "/IT-full/"+pathSegment(vatCodeOrTaxCode), nil)
}

func (c *CompanyClient) CreateITFullRequest(ctx context.Context, vatCodeOrTaxCode string, body CompanyPostBody) (Envelope[CompanyRequest], error) {
	return c.postRequest(ctx, "/IT-full/"+pathSegment(vatCodeOrTaxCode), body)
}

func (c *CompanyClient) CheckITRequest(ctx context.Context, id string) (Envelope[CompanyDataset], error) {
	return c.getDataset(ctx, "/IT-check_id/"+pathSegment(id), nil)
}

func (c *CompanyClient) GetITStart(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-start/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetITAdvanced(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-advanced/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) SearchIT(ctx context.Context, params CompanyITSearchParams) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-search", params.values())
}

func (c *CompanyClient) SearchITRaw(ctx context.Context, params CompanyITSearchParams) (Envelope[CompanyDataset], error) {
	return c.getDataset(ctx, "/IT-search", params.values())
}

func (c *CompanyClient) GetITShareholders(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[CompanyDataset], error) {
	return c.getDataset(ctx, "/IT-shareholders/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetITAddress(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-address/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetITPEC(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-pec/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetITClosed(ctx context.Context, vatCodeTaxCodeOrID string, params CompanyClosedParams) (Envelope[[]CompanyDataset], error) {
	query := url.Values{}
	setQueryValue(query, "date", params.Date)
	return c.getDatasetList(ctx, "/IT-closed/"+pathSegment(vatCodeTaxCodeOrID), query)
}

func (c *CompanyClient) GetITVATGroup(ctx context.Context, vatCodeOrID string, params CompanyVATGroupParams) (Envelope[[]CompanyDataset], error) {
	query := url.Values{}
	setQueryValue(query, "taxCode", params.TaxCode)
	return c.getDatasetList(ctx, "/IT-vatgroup/"+pathSegment(vatCodeOrID), query)
}

func (c *CompanyClient) GetITSDICode(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-sdicode/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) ListITLegalForms(ctx context.Context) (Envelope[[]CompanyLegalForm], error) {
	var out Envelope[[]CompanyLegalForm]
	err := c.client.getCompany(ctx, "/IT-legalforms", nil, &out)
	return out, err
}

func (c *CompanyClient) GetITLegalForm(ctx context.Context, code string) (Envelope[[]CompanyLegalForm], error) {
	var out Envelope[[]CompanyLegalForm]
	err := c.client.getCompany(ctx, "/IT-legalforms/"+pathSegment(code), nil, &out)
	return out, err
}

func (c *CompanyClient) GetEUStart(ctx context.Context, vatCodeTaxCode string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/EU-start/"+pathSegment(vatCodeTaxCode), nil)
}

func (c *CompanyClient) GetITSplitPayment(ctx context.Context, taxCodeVATCode string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-splitpayment/"+pathSegment(taxCodeVATCode), nil)
}

func (c *CompanyClient) GetITPA(ctx context.Context, taxCodeVATCode string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-pa/"+pathSegment(taxCodeVATCode), nil)
}

func (c *CompanyClient) GetITName(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-name/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetITUBO(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/IT-ubo/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetFRStart(ctx context.Context, siretSirenVATCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/FR-start/"+pathSegment(siretSirenVATCodeOrID), nil)
}

func (c *CompanyClient) GetFRAdvanced(ctx context.Context, siretSirenVATCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/FR-advanced/"+pathSegment(siretSirenVATCodeOrID), nil)
}

func (c *CompanyClient) SearchFR(ctx context.Context, params CompanyFRSearchParams) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/FR-search", params.values())
}

func (c *CompanyClient) GetDEStart(ctx context.Context, vatCodeCompanyNumberOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/DE-start/"+pathSegment(vatCodeCompanyNumberOrID), nil)
}

func (c *CompanyClient) GetDEAdvanced(ctx context.Context, vatCodeCompanyNumberOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/DE-advanced/"+pathSegment(vatCodeCompanyNumberOrID), nil)
}

func (c *CompanyClient) GetESStart(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/ES-start/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetESAdvanced(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/ES-advanced/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetPTStart(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/PT-start/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetPTAdvanced(ctx context.Context, vatCodeTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/PT-advanced/"+pathSegment(vatCodeTaxCodeOrID), nil)
}

func (c *CompanyClient) GetGBStart(ctx context.Context, vatCodeCompanyNumberOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/GB-start/"+pathSegment(vatCodeCompanyNumberOrID), nil)
}

func (c *CompanyClient) GetGBAdvanced(ctx context.Context, vatCodeCompanyNumberOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/GB-advanced/"+pathSegment(vatCodeCompanyNumberOrID), nil)
}

func (c *CompanyClient) GetWWStart(ctx context.Context, country string, vatCodeCompanyNumberTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/WW-start/"+pathSegment(country)+"/"+pathSegment(vatCodeCompanyNumberTaxCodeOrID), nil)
}

func (c *CompanyClient) GetWWAdvanced(ctx context.Context, country string, vatCodeCompanyNumberTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/WW-advanced/"+pathSegment(country)+"/"+pathSegment(vatCodeCompanyNumberTaxCodeOrID), nil)
}

func (c *CompanyClient) GetWWTop(ctx context.Context, country string, vatCodeCompanyNumberTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/WW-top/"+pathSegment(country)+"/"+pathSegment(vatCodeCompanyNumberTaxCodeOrID), nil)
}

func (c *CompanyClient) GetBEStart(ctx context.Context, vatCodeCompanyNumberTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/BE-start/"+pathSegment(vatCodeCompanyNumberTaxCodeOrID), nil)
}

func (c *CompanyClient) GetBEAdvanced(ctx context.Context, vatCodeCompanyNumberTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/BE-advanced/"+pathSegment(vatCodeCompanyNumberTaxCodeOrID), nil)
}

func (c *CompanyClient) GetATStart(ctx context.Context, vatCodeCompanyNumberOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/AT-start/"+pathSegment(vatCodeCompanyNumberOrID), nil)
}

func (c *CompanyClient) GetATAdvanced(ctx context.Context, vatCodeCompanyNumberOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/AT-advanced/"+pathSegment(vatCodeCompanyNumberOrID), nil)
}

func (c *CompanyClient) GetCHStart(ctx context.Context, vatCodeCompanyNumberTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/CH-start/"+pathSegment(vatCodeCompanyNumberTaxCodeOrID), nil)
}

func (c *CompanyClient) GetCHAdvanced(ctx context.Context, vatCodeCompanyNumberTaxCodeOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/CH-advanced/"+pathSegment(vatCodeCompanyNumberTaxCodeOrID), nil)
}

func (c *CompanyClient) GetPLStart(ctx context.Context, vatCodeCompanyNumberTaxCodeREGONNumberOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/PL-start/"+pathSegment(vatCodeCompanyNumberTaxCodeREGONNumberOrID), nil)
}

func (c *CompanyClient) GetPLAdvanced(ctx context.Context, vatCodeCompanyNumberTaxCodeREGONNumberOrID string) (Envelope[[]CompanyDataset], error) {
	return c.getDatasetList(ctx, "/PL-advanced/"+pathSegment(vatCodeCompanyNumberTaxCodeREGONNumberOrID), nil)
}

func (c *CompanyClient) ListMonitors(ctx context.Context) (Envelope[[]CompanyMonitorSummary], error) {
	var out Envelope[[]CompanyMonitorSummary]
	err := c.client.getCompany(ctx, "/monitor", nil, &out)
	return out, err
}

func (c *CompanyClient) CreateMonitor(ctx context.Context, payload CompanyMonitorPayload) (Envelope[CompanyMonitorRequest], error) {
	var out Envelope[CompanyMonitorRequest]
	err := c.client.postCompany(ctx, "/monitor", payload, &out)
	return out, err
}

func (c *CompanyClient) GetMonitor(ctx context.Context, id string, params CompanyMonitorGetParams) (Envelope[CompanyMonitorRequest], error) {
	query := url.Values{}
	if params.Callback {
		query.Set("callback", "")
	}
	var out Envelope[CompanyMonitorRequest]
	err := c.client.getCompany(ctx, "/monitor/"+pathSegment(id), query, &out)
	return out, err
}

func (c *CompanyClient) DeleteMonitor(ctx context.Context, id string) (Envelope[[]string], error) {
	var out Envelope[[]string]
	err := c.client.deleteCompany(ctx, "/monitor/"+pathSegment(id), &out)
	return out, err
}

func (c *CompanyClient) getDataset(ctx context.Context, path string, query url.Values) (Envelope[CompanyDataset], error) {
	var out Envelope[CompanyDataset]
	err := c.client.getCompany(ctx, path, query, &out)
	return out, err
}

func (c *CompanyClient) getDatasetList(ctx context.Context, path string, query url.Values) (Envelope[[]CompanyDataset], error) {
	var out Envelope[[]CompanyDataset]
	err := c.client.getCompany(ctx, path, query, &out)
	return out, err
}

func (c *CompanyClient) postRequest(ctx context.Context, path string, body CompanyPostBody) (Envelope[CompanyRequest], error) {
	var out Envelope[CompanyRequest]
	err := c.client.postCompany(ctx, path, body, &out)
	return out, err
}

func (p CompanyITSearchParams) Values() url.Values {
	return p.values()
}

func (p CompanyITSearchParams) values() url.Values {
	query := url.Values{}
	setIntQueryValue(query, "dryRun", p.DryRun)
	setQueryValue(query, "dataEnrichment", p.DataEnrichment)
	setFloatQueryValue(query, "lat", p.Lat)
	setFloatQueryValue(query, "long", p.Long)
	setIntQueryValue(query, "radius", p.Radius)
	setQueryValue(query, "companyName", p.CompanyName)
	setQueryValue(query, "autocomplete", p.Autocomplete)
	setQueryValue(query, "province", p.Province)
	setQueryValue(query, "townCode", p.TownCode)
	setQueryValue(query, "atecoCode", p.AtecoCode)
	setQueryValue(query, "cciaa", p.CCIAA)
	setQueryValue(query, "reaCode", p.REACode)
	setIntQueryValue(query, "minTurnover", p.MinTurnover)
	setIntQueryValue(query, "maxTurnover", p.MaxTurnover)
	setIntQueryValue(query, "minEmployees", p.MinEmployees)
	setIntQueryValue(query, "maxEmployees", p.MaxEmployees)
	setQueryValue(query, "sdiCode", p.SDICode)
	setQueryValue(query, "legalFormCode", p.LegalFormCode)
	setQueryValue(query, "shareHolderTaxCode", p.ShareHolderTaxCode)
	setQueryValue(query, "activityStatus", p.ActivityStatus)
	setQueryValue(query, "pec", p.PEC)
	setIntQueryValue(query, "creationTimestamp", p.CreationTimestamp)
	setIntQueryValue(query, "lastUpdateTimestamp", p.LastUpdateTimestamp)
	setIntQueryValue(query, "skip", p.Skip)
	setIntQueryValue(query, "limit", p.Limit)
	return query
}

func (p CompanyFRSearchParams) values() url.Values {
	query := url.Values{}
	setIntQueryValue(query, "dryRun", p.DryRun)
	setQueryValue(query, "dataEnrichment", p.DataEnrichment)
	setFloatQueryValue(query, "lat", p.Lat)
	setFloatQueryValue(query, "long", p.Long)
	setIntQueryValue(query, "radius", p.Radius)
	setQueryValue(query, "companyName", p.CompanyName)
	setQueryValue(query, "autocomplete", p.Autocomplete)
	setQueryValue(query, "town", p.Town)
	setQueryValue(query, "townCode", p.TownCode)
	setQueryValue(query, "nafCode", p.NAFCode)
	setQueryValue(query, "departmentCode", p.DepartmentCode)
	setQueryValue(query, "regionCode", p.RegionCode)
	setIntQueryValue(query, "minTurnover", p.MinTurnover)
	setIntQueryValue(query, "maxTurnover", p.MaxTurnover)
	setIntQueryValue(query, "minEmployees", p.MinEmployees)
	setIntQueryValue(query, "maxEmployees", p.MaxEmployees)
	setIntQueryValue(query, "creditWorthy", p.CreditWorthy)
	setQueryValue(query, "legalFormCode", p.LegalFormCode)
	setQueryValue(query, "activityStatus", p.ActivityStatus)
	setIntQueryValue(query, "creationTimestamp", p.CreationTimestamp)
	setIntQueryValue(query, "lastUpdateTimestamp", p.LastUpdateTimestamp)
	setIntQueryValue(query, "skip", p.Skip)
	setIntQueryValue(query, "limit", p.Limit)
	return query
}

func setIntQueryValue(query url.Values, key string, value *int) {
	if value != nil {
		query.Set(key, strconv.Itoa(*value))
	}
}

func setFloatQueryValue(query url.Values, key string, value *float64) {
	if value != nil {
		query.Set(key, strconv.FormatFloat(*value, 'f', -1, 64))
	}
}
