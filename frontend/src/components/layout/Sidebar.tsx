import { useI18n } from '../../i18n/I18nProvider'
import type { User } from '../../types'
import { LanguageSwitcher } from './LanguageSwitcher'

type SidebarProps = {
	user: User
	onLogout: () => void
	onBack: () => void
}

export function Sidebar({ user, onLogout, onBack }: SidebarProps) {
	const isAdmin = user.role === 'admin'
	const { t } = useI18n()

	return (
		<aside>
			<div className='brand' onClick={onBack}>
				<span>TM</span>
				<strong>{t('brand')}</strong>
			</div>

			<LanguageSwitcher />

			<nav>
				<a className='active' href='#dashboard'>
					{t('dashboardNav')}
				</a>

				<a href='#scans'>{t('historyNav')}</a>
			</nav>

			<div className='account-info'>
				<strong>{user.email}</strong>
				<small>{isAdmin ? t('administrator') : t('user')}</small>
			</div>

			<button className='logout' onClick={onLogout}>
				{t('logout')}
			</button>
		</aside>
	)
}
