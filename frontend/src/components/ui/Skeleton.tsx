type SkeletonProps = {
	width?: string
	height?: string
	radius?: string
	className?: string
}

export function Skeleton({
	width = '100%',
	height = '16px',
	radius = '7px',
	className = '',
}: SkeletonProps) {
	return (
		<span
			className={`skeleton ${className}`}
			style={{ width, height, borderRadius: radius }}
			aria-hidden='true'
		/>
	)
}

export function MetricCardSkeleton() {
	return (
		<article className='metric-skeleton' aria-busy='true'>
			<Skeleton width='55%' height='13px' />
			<Skeleton width='35%' height='34px' />
			<Skeleton width='70%' height='12px' />
		</article>
	)
}

export function ScanTableSkeleton({ rows = 5 }: { rows?: number }) {
	return (
		<>
			{Array.from({ length: rows }, (_, index) => (
				<tr key={index} className='skeleton-row' aria-hidden='true'>
					<td>
						<Skeleton width='230px' height='14px' />
						<Skeleton width='150px' height='10px' />
					</td>
					<td>
						<Skeleton width='85px' height='23px' />
					</td>
					<td>
						<Skeleton width='85px' height='23px' />
					</td>
					<td>
						<Skeleton width='45px' height='14px' />
					</td>
					<td>
						<Skeleton width='120px' height='14px' />
					</td>
				</tr>
			))}
		</>
	)
}

export function RiskDistributionSkeleton() {
	return (
		<section className='risk-distribution' aria-busy='true'>
			<Skeleton width='230px' height='20px' />
			<div className='risk-distribution-list'>
				{Array.from({ length: 5 }, (_, index) => (
					<div className='risk-skeleton-row' key={index}>
						<Skeleton width='100px' height='20px' />
						<Skeleton height='9px' />
						<Skeleton width='35px' height='13px' />
					</div>
				))}
			</div>
		</section>
	)
}

export function ScanDetailsSkeleton() {
	return (
		<div className='scan-details-skeleton' aria-busy='true'>
			<div className='details-skeleton-heading'>
				<Skeleton width='90px' height='12px' />
				<Skeleton width='360px' height='30px' />
				<Skeleton width='520px' height='15px' />
			</div>

			<div className='details-skeleton-metrics'>
				{Array.from({ length: 3 }, (_, index) => (
					<div className='details-skeleton-card' key={index}>
						<Skeleton width='90px' height='13px' />
						<Skeleton width='65px' height='30px' />
						<Skeleton height='8px' />
					</div>
				))}
			</div>

			<div className='details-skeleton-card'>
				<Skeleton width='200px' height='20px' />
				{Array.from({ length: 5 }, (_, index) => (
					<Skeleton key={index} height='15px' />
				))}
			</div>

			<div className='details-skeleton-card'>
				<Skeleton width='220px' height='20px' />
				{Array.from({ length: 3 }, (_, index) => (
					<Skeleton key={index} height='55px' />
				))}
			</div>
		</div>
	)
}
