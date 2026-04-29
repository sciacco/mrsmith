import { Button, Icon, Modal, SingleSelect, provinceSelectOptions, useToast } from '@mrsmith/ui';
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { useCountries, useProviderMutations } from '../api/queries';
import type { Country, ProviderPayload, ProviderSummary } from '../api/types';
import { apiErrorMessage } from '../lib/api-error';
import styles from './ProviderRequestModal.module.css';

interface ProviderRequestModalProps {
  open: boolean;
  initialCompanyName: string;
  onClose: () => void;
  onCreated: (provider: ProviderSummary) => void;
}

interface FieldErrors {
  company_name?: string;
  tax_id?: string;
  address?: string;
  city?: string;
  postal_code?: string;
  province?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
}

const LANGUAGE_OPTIONS = [
  { value: 'it', label: 'Italiano' },
  { value: 'en', label: 'Inglese' },
];

const COUNTRY_FALLBACK_LABELS: Record<string, string> = {
  IT: 'Italia',
};

function countrySelectOptions(countries: Country[] | undefined, current?: string | null) {
  const options = countries?.map((item) => ({ value: item.code, label: item.name })) ?? [];
  if (current && !options.some((item) => item.value === current)) {
    return [{ value: current, label: COUNTRY_FALLBACK_LABELS[current] ?? current }, ...options];
  }
  return options;
}

function firstError(errors: FieldErrors) {
  return (
    errors.company_name ??
    errors.tax_id ??
    errors.address ??
    errors.city ??
    errors.postal_code ??
    errors.province ??
    errors.first_name ??
    errors.last_name ??
    errors.email ??
    null
  );
}

export function ProviderRequestModal({ open, initialCompanyName, onClose, onCreated }: ProviderRequestModalProps) {
  const countriesQuery = useCountries();
  const { createProvider } = useProviderMutations();
  const { toast } = useToast();
  const formRef = useRef<HTMLFormElement>(null);
  const [country, setCountry] = useState('IT');
  const [language, setLanguage] = useState('it');
  const [province, setProvince] = useState('');
  const [errors, setErrors] = useState<FieldErrors>({});
  const countryOptions = useMemo(() => countrySelectOptions(countriesQuery.data, country), [countriesQuery.data, country]);
  const provinceOptions = useMemo(() => provinceSelectOptions(province), [province]);

  useEffect(() => {
    if (!open) return;
    const form = formRef.current;
    form?.reset();
    setCountry('IT');
    setLanguage('it');
    setProvince('');
    setErrors({});

    const companyInput = form?.elements.namedItem('company_name');
    if (companyInput instanceof HTMLInputElement) {
      companyInput.value = initialCompanyName.trim();
      companyInput.focus();
    }
  }, [initialCompanyName, open]);

  function closeModal() {
    if (createProvider.isPending) return;
    onClose();
  }

  function clearError(...names: (keyof FieldErrors)[]) {
    setErrors((current) => {
      const next = { ...current };
      for (const name of names) next[name] = undefined;
      return next;
    });
  }

  function changeCountry(nextCountry: string | null) {
    setCountry(nextCountry || 'IT');
    setProvince('');
    clearError('tax_id', 'postal_code', 'province');
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const formData = new FormData(form);
    const companyName = String(formData.get('company_name') ?? '').trim();
    const vatNumber = String(formData.get('vat_number') ?? '').trim();
    const cf = String(formData.get('cf') ?? '').trim();
    const address = String(formData.get('address') ?? '').trim();
    const city = String(formData.get('city') ?? '').trim();
    const postalCode = String(formData.get('postal_code') ?? '').trim();
    const selectedCountry = String(formData.get('country') || 'IT');
    const selectedLanguage = String(formData.get('language') || 'it');
    const selectedProvince = String(formData.get('province') ?? '').trim();
    const firstName = String(formData.get('first_name') ?? '').trim();
    const lastName = String(formData.get('last_name') ?? '').trim();
    const email = String(formData.get('email') ?? '').trim();
    const nextErrors: FieldErrors = {};

    if (!companyName) nextErrors.company_name = 'Inserisci la ragione sociale';
    if (!address) nextErrors.address = "Inserisci l'indirizzo";
    if (!city) nextErrors.city = 'Inserisci la citta';
    if (selectedCountry === 'IT') {
      if (!vatNumber && !cf) nextErrors.tax_id = 'Per i fornitori italiani inserisci CF o P.IVA';
      if (postalCode.length < 5) nextErrors.postal_code = 'CAP non valido';
      if (!selectedProvince) nextErrors.province = 'Seleziona la provincia';
    }
    if (!firstName) nextErrors.first_name = 'Inserisci il nome';
    if (!lastName) nextErrors.last_name = 'Inserisci il cognome';
    if (!email) nextErrors.email = "Inserisci l'email del contatto qualifica";

    const validationMessage = firstError(nextErrors);
    if (validationMessage) {
      setErrors(nextErrors);
      toast(validationMessage, 'warning');
      const fieldName = nextErrors.company_name
        ? 'company_name'
        : nextErrors.tax_id
          ? 'vat_number'
          : nextErrors.address
            ? 'address'
            : nextErrors.city
              ? 'city'
              : nextErrors.postal_code
                ? 'postal_code'
                : nextErrors.province
                  ? 'province'
                  : nextErrors.first_name
                    ? 'first_name'
                    : nextErrors.last_name
                      ? 'last_name'
                      : 'email';
      const field = form.elements.namedItem(fieldName);
      if (field instanceof HTMLElement) field.focus();
      return;
    }

    const payload: ProviderPayload = {
      company_name: companyName,
      state: 'DRAFT',
      country: selectedCountry,
      language: selectedLanguage,
      vat_number: vatNumber || undefined,
      cf: cf || undefined,
      postal_code: postalCode || undefined,
      province: selectedProvince || undefined,
      city,
      address,
      ref: {
        first_name: firstName,
        last_name: lastName,
        email,
        phone: String(formData.get('phone') ?? '').trim(),
        reference_type: 'QUALIFICATION_REF',
      },
    };

    try {
      const provider = await createProvider.mutateAsync(payload);
      toast('Richiesta di censimento creata');
      form.reset();
      setCountry('IT');
      setLanguage('it');
      setProvince('');
      setErrors({});
      onCreated(provider);
      onClose();
    } catch (error) {
      toast(apiErrorMessage(error, 'Dati fornitore non disponibili in questo momento'), 'error');
    }
  }

  return (
    <Modal open={open} onClose={closeModal} title="Nuovo fornitore" size="xwide" dismissible={!createProvider.isPending}>
      <form ref={formRef} className={styles.form} noValidate onSubmit={(event) => void submit(event)}>
        <fieldset className={styles.section}>
          <legend>Azienda</legend>
          <div className={`${styles.sectionGrid} ${styles.companyGrid}`}>
            <div className={`field ${styles.span2}`}>
              <label>Ragione sociale</label>
              <input
                name="company_name"
                required
                aria-invalid={Boolean(errors.company_name)}
                onChange={() => clearError('company_name')}
              />
              {errors.company_name ? <p className="fieldError">{errors.company_name}</p> : null}
            </div>

            <div className="field">
              <label>P.IVA</label>
              <input
                name="vat_number"
                aria-describedby={errors.tax_id ? 'provider-tax-error' : undefined}
                aria-invalid={Boolean(errors.tax_id)}
                onChange={() => clearError('tax_id')}
              />
            </div>
            <div className="field">
              <label>CF</label>
              <input
                name="cf"
                aria-describedby={errors.tax_id ? 'provider-tax-error' : undefined}
                aria-invalid={Boolean(errors.tax_id)}
                onChange={() => clearError('tax_id')}
              />
            </div>

            <div className={`field ${styles.countryField}`}>
              <label>Paese</label>
              <SingleSelect<string>
                options={countryOptions}
                selected={country}
                onChange={changeCountry}
                placeholder="Seleziona paese"
                disabled={countriesQuery.isLoading || countriesQuery.isError}
              />
              <input type="hidden" name="country" value={country} />
            </div>
            <div className={`field ${styles.languageField}`}>
              <label>Lingua</label>
              <SingleSelect<string>
                options={LANGUAGE_OPTIONS}
                selected={language}
                onChange={(nextLanguage) => setLanguage(nextLanguage || 'it')}
                placeholder="Seleziona lingua"
              />
              <input type="hidden" name="language" value={language} />
            </div>
            {errors.tax_id || countriesQuery.isError ? (
              <div className={styles.companyErrorRow}>
                {errors.tax_id ? <p id="provider-tax-error" className={`fieldError ${styles.companyTaxError}`}>{errors.tax_id}</p> : <span className={styles.companyTaxError} />}
                {countriesQuery.isError ? <p className={`fieldError ${styles.companyCountryError}`}>Elenco paesi non disponibile</p> : <span className={styles.companyCountryError} />}
              </div>
            ) : null}
          </div>
        </fieldset>

        <fieldset className={styles.section}>
          <legend>Sede aziendale</legend>
          <div className={`${styles.sectionGrid} ${styles.siteGrid}`}>
            <div className={`field ${styles.span2}`}>
              <label>Indirizzo</label>
              <input
                name="address"
                required
                aria-invalid={Boolean(errors.address)}
                onChange={() => clearError('address')}
              />
              {errors.address ? <p className="fieldError">{errors.address}</p> : null}
            </div>
            <div className={`field ${styles.cityField}`}>
              <label>Citta</label>
              <input
                name="city"
                required
                aria-invalid={Boolean(errors.city)}
                onChange={() => clearError('city')}
              />
            </div>
            <div className={`field ${styles.postalCodeField}`}>
              <label>CAP</label>
              <input
                name="postal_code"
                aria-invalid={Boolean(errors.postal_code)}
                onChange={() => clearError('postal_code')}
              />
            </div>
            <div className={`field ${styles.provinceField}`}>
              <label>{country === 'IT' ? 'Provincia' : 'Provincia / Stato'}</label>
              {country === 'IT' ? (
                <>
                  <SingleSelect<string>
                    options={provinceOptions}
                    selected={province || null}
                    onChange={(nextProvince) => {
                      setProvince(nextProvince || '');
                      clearError('province');
                    }}
                    placeholder="Seleziona provincia"
                  />
                  <input type="hidden" name="province" value={province} />
                </>
              ) : (
                <input
                  name="province"
                  value={province}
                  onChange={(event) => {
                    setProvince(event.target.value);
                    clearError('province');
                  }}
                />
              )}
            </div>
            {errors.city || errors.postal_code || errors.province ? (
              <div className={styles.siteErrorRow}>
                {errors.city ? <p className={`fieldError ${styles.siteCityError}`}>{errors.city}</p> : <span className={styles.siteCityError} />}
                {errors.postal_code ? <p className={`fieldError ${styles.sitePostalError}`}>{errors.postal_code}</p> : <span className={styles.sitePostalError} />}
                {errors.province ? <p className={`fieldError ${styles.siteProvinceError}`}>{errors.province}</p> : <span className={styles.siteProvinceError} />}
              </div>
            ) : null}
          </div>
        </fieldset>

        <fieldset className={`${styles.section} ${styles.contactSection}`}>
          <legend>Referente qualifica</legend>
          <div className={`${styles.sectionGrid} ${styles.contactGrid}`}>
            <div className={`field ${styles.contactFirstField}`}>
              <label>Nome</label>
              <input
                name="first_name"
                required
                aria-invalid={Boolean(errors.first_name)}
                onChange={() => clearError('first_name')}
              />
            </div>
            <div className={`field ${styles.contactLastField}`}>
              <label>Cognome</label>
              <input
                name="last_name"
                required
                aria-invalid={Boolean(errors.last_name)}
                onChange={() => clearError('last_name')}
              />
            </div>
            {errors.first_name || errors.last_name ? (
              <div className={styles.contactErrorRow}>
                {errors.first_name ? <p className={`fieldError ${styles.contactFirstError}`}>{errors.first_name}</p> : <span className={styles.contactFirstError} />}
                {errors.last_name ? <p className={`fieldError ${styles.contactLastError}`}>{errors.last_name}</p> : <span className={styles.contactLastError} />}
              </div>
            ) : null}
            <div className={`field ${styles.contactEmailField}`}>
              <label>Email qualifica</label>
              <input
                name="email"
                type="email"
                required
                aria-invalid={Boolean(errors.email)}
                onChange={() => clearError('email')}
              />
            </div>
            <div className={`field ${styles.contactPhoneField}`}>
              <label>Telefono</label>
              <input name="phone" />
            </div>
            {errors.email ? (
              <div className={styles.contactErrorRow}>
                <p className={`fieldError ${styles.contactEmailError}`}>{errors.email}</p>
              </div>
            ) : null}
          </div>
        </fieldset>

        <div className={`modalActions ${styles.actions}`}>
          <Button type="button" variant="secondary" disabled={createProvider.isPending} onClick={closeModal}>
            Annulla
          </Button>
          <Button type="submit" leftIcon={<Icon name="check" />} loading={createProvider.isPending}>
            Invia richiesta di censimento
          </Button>
        </div>
      </form>
    </Modal>
  );
}
