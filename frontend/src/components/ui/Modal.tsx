import { FC, MouseEvent } from 'react'
import { useI18n } from '../../i18n/I18nProvider'

interface IModal {
	onOk: () => void
	onCancel: () => void
	isModal: boolean
}

export const Modal: FC<IModal> = ({ isModal, onOk, onCancel }) => {
	const { t } = useI18n()

	const handleBackdropClick = (event: MouseEvent<HTMLDivElement>) => {
		if (event.target === event.currentTarget) {
			onCancel()
		}
	}
	return (
		<div className='modal-backdrop' onClick={handleBackdropClick}>
			<div className='modal' role='dialog' aria-modal='true'>
				<h1>{t('logout')}</h1>
				<div className='modal-row'>
					<button type='button' onClick={onOk}>
						{t('yes')}
					</button>
					<button type='button' onClick={onCancel}>
						{t('no')}
					</button>
				</div>
			</div>
		</div>
	)
}
