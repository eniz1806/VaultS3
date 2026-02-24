interface BarChartItem {
  label: string
  value: number
  color?: string
}

interface BarChartProps {
  items: BarChartItem[]
  height?: number
  formatValue?: (v: number) => string
}

const COLORS = [
  '#6366f1', '#8b5cf6', '#a855f7', '#d946ef',
  '#ec4899', '#f43f5e', '#f97316', '#eab308',
  '#22c55e', '#14b8a6', '#06b6d4', '#3b82f6',
]

export default function BarChart({ items, height = 200, formatValue }: BarChartProps) {
  if (items.length === 0) return null

  const maxVal = Math.max(...items.map(i => i.value), 1)
  const barWidth = Math.min(40, Math.max(16, Math.floor(300 / items.length)))
  const gap = Math.min(12, Math.max(4, Math.floor(80 / items.length)))
  const chartWidth = items.length * (barWidth + gap) + gap
  const labelHeight = 40
  const topPad = 20
  const barArea = height - labelHeight - topPad

  return (
    <svg
      viewBox={`0 0 ${chartWidth} ${height}`}
      className="w-full"
      style={{ maxHeight: height }}
    >
      {items.map((item, i) => {
        const barH = Math.max(2, (item.value / maxVal) * barArea)
        const x = gap + i * (barWidth + gap)
        const y = topPad + barArea - barH
        const color = item.color || COLORS[i % COLORS.length]

        return (
          <g key={i}>
            {/* Value label */}
            <text
              x={x + barWidth / 2}
              y={y - 4}
              textAnchor="middle"
              className="fill-gray-500 dark:fill-gray-400"
              fontSize={9}
            >
              {formatValue ? formatValue(item.value) : item.value}
            </text>

            {/* Bar */}
            <rect
              x={x}
              y={y}
              width={barWidth}
              height={barH}
              rx={3}
              fill={color}
              opacity={0.85}
            />

            {/* Label */}
            <text
              x={x + barWidth / 2}
              y={height - labelHeight + 14}
              textAnchor="middle"
              className="fill-gray-600 dark:fill-gray-400"
              fontSize={10}
              fontWeight={500}
            >
              {item.label.length > 8 ? item.label.slice(0, 7) + '..' : item.label}
            </text>
          </g>
        )
      })}
    </svg>
  )
}
