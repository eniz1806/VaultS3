interface DonutChartItem {
  label: string
  value: number
  color?: string
}

interface DonutChartProps {
  items: DonutChartItem[]
  size?: number
  thickness?: number
}

const COLORS = [
  '#6366f1', '#8b5cf6', '#a855f7', '#d946ef',
  '#ec4899', '#f43f5e', '#f97316', '#eab308',
  '#22c55e', '#14b8a6', '#06b6d4', '#3b82f6',
]

export default function DonutChart({ items, size = 160, thickness = 28 }: DonutChartProps) {
  const total = items.reduce((s, i) => s + i.value, 0)
  if (total === 0) return null

  const cx = size / 2
  const cy = size / 2
  const radius = (size - thickness) / 2

  let currentAngle = -90 // start at top

  const segments = items.map((item, i) => {
    const pct = item.value / total
    const angle = pct * 360
    const startAngle = currentAngle
    const endAngle = currentAngle + angle
    currentAngle = endAngle

    const startRad = (startAngle * Math.PI) / 180
    const endRad = (endAngle * Math.PI) / 180

    const x1 = cx + radius * Math.cos(startRad)
    const y1 = cy + radius * Math.sin(startRad)
    const x2 = cx + radius * Math.cos(endRad)
    const y2 = cy + radius * Math.sin(endRad)

    const largeArc = angle > 180 ? 1 : 0
    const color = item.color || COLORS[i % COLORS.length]

    // For single item, draw full circle
    if (items.length === 1) {
      return (
        <circle
          key={i}
          cx={cx}
          cy={cy}
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth={thickness}
          opacity={0.85}
        />
      )
    }

    const d = `M ${x1} ${y1} A ${radius} ${radius} 0 ${largeArc} 1 ${x2} ${y2}`

    return (
      <path
        key={i}
        d={d}
        fill="none"
        stroke={color}
        strokeWidth={thickness}
        strokeLinecap="round"
        opacity={0.85}
      />
    )
  })

  return (
    <div className="flex items-center gap-4">
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
        {segments}
        {/* Center text */}
        <text
          x={cx}
          y={cy - 6}
          textAnchor="middle"
          className="fill-gray-900 dark:fill-white"
          fontSize={18}
          fontWeight={700}
        >
          {total.toLocaleString()}
        </text>
        <text
          x={cx}
          y={cy + 12}
          textAnchor="middle"
          className="fill-gray-500 dark:fill-gray-400"
          fontSize={10}
        >
          total
        </text>
      </svg>

      {/* Legend */}
      <div className="flex flex-col gap-1.5">
        {items.map((item, i) => (
          <div key={i} className="flex items-center gap-2 text-xs">
            <span
              className="w-2.5 h-2.5 rounded-full flex-shrink-0"
              style={{ backgroundColor: item.color || COLORS[i % COLORS.length] }}
            />
            <span className="text-gray-700 dark:text-gray-300">{item.label}</span>
            <span className="text-gray-400 dark:text-gray-500 ml-auto font-mono">
              {item.value.toLocaleString()}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}
