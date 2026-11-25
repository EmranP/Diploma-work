import { useI18n } from '../../i18n/I18nProvider'

export function LanguageSwitcher() {
	const { language, setLanguage } = useI18n()

	return (
		<label className='language-control'>
			<span>Language</span>

			<select
				className='language-switcher'
				value={language}
				onChange={event => setLanguage(event.target.value as 'en' | 'tk')}
				aria-label='Select language'
			>
				<option value='tk'>Türkmençe</option>
				<option value='en'>English</option>
			</select>
		</label>
	)
}
