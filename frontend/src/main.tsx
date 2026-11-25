import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import { App } from './App'
import { ToastProvider } from './components/ui/ToastProvider'
import { I18nProvider } from './i18n/I18nProvider'
import './styles.css'

const rootElement = document.getElementById('root')

if (!rootElement) {
	throw new Error('Root element was not found')
}

createRoot(rootElement).render(
	<StrictMode>
		<I18nProvider>
			<ToastProvider>
				<App />
			</ToastProvider>
		</I18nProvider>
	</StrictMode>
)
