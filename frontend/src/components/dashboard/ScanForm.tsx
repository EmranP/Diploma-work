import { type FormEvent, useState } from 'react'

import { useI18n } from '../../i18n/I18nProvider'
import { Spinner } from '../ui/Spinner'

type ScanFormProps = {
	loading: boolean
	onSubmit: (url: string) => Promise<boolean>
}

export function ScanForm({ loading, onSubmit }: ScanFormProps) {
	const { t } = useI18n()
	const [url, setURL] = useState('')

	async function submit(event: FormEvent<HTMLFormElement>) {
		event.preventDefault()

		const value = url.trim()

		if (!value || loading) return

		const success = await onSubmit(value)

		if (success) {
			setURL('')
		}
	}

	return (
		<section className='scan-card'>
			<div>
				<h2>{t('scanURLTitle')}</h2>
				<p>{t('scanURLDescription')}</p>
			</div>

			<form onSubmit={submit}>
				<input
					value={url}
					onChange={event => setURL(event.target.value)}
					placeholder={t('scanURLPlaceholder')}
					aria-label={t('url')}
					required
					disabled={loading}
				/>

				<button type='submit' disabled={loading}>
					{loading ? (
						<>
							<Spinner />
							{t('sending')}
						</>
					) : (
						t('checkURL')
					)}
				</button>
			</form>
		</section>
	)
}
