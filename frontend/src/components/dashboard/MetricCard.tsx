import { Skeleton } from '../ui/Skeleton'

type MetricCardProps = {
	title: string
	value: string | number
	description: string
	loading?: boolean
}

export function MetricCard({
	title,
	value,
	description,
	loading = false,
}: MetricCardProps) {
	if (loading) {
		return (
			<article className='metric-skeleton' aria-busy='true'>
				<Skeleton width='55%' height='13px' />
				<Skeleton width='35%' height='34px' />
				<Skeleton width='70%' height='12px' />
			</article>
		)
	}

	return (
		<article>
			<span>{title}</span>
			<strong>{value}</strong>
			<small>{description}</small>
		</article>
	)
}
