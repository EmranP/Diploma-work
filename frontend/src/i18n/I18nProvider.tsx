import {
	createContext,
	useCallback,
	useContext,
	useMemo,
	useState,
	type ReactNode,
} from 'react'

import {
	translations,
	type Language,
	type TranslationKey,
} from './translations'

const LANGUAGE_KEY = 'phishing-detector.language'

type I18nContextValue = {
	language: Language
	setLanguage: (language: Language) => void
	t: (key: TranslationKey) => string
}

const I18nContext = createContext<I18nContextValue | null>(null)

function getInitialLanguage(): Language {
	const value = localStorage.getItem(LANGUAGE_KEY)

	return value === 'en' ? 'en' : 'tk'
}

export function I18nProvider({ children }: { children: ReactNode }) {
	const [language, setLanguageState] = useState<Language>(getInitialLanguage)

	const setLanguage = useCallback((next: Language) => {
		localStorage.setItem(LANGUAGE_KEY, next)
		setLanguageState(next)
	}, [])

	const t = useCallback(
		(key: TranslationKey) => translations[language][key],
		[language]
	)

	const value = useMemo(
		() => ({
			language,
			setLanguage,
			t,
		}),
		[language, setLanguage, t]
	)

	return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n(): I18nContextValue {
	const context = useContext(I18nContext)

	if (!context) {
		throw new Error('useI18n must be used inside I18nProvider')
	}

	return context
}
