interface SparklineProps {
  data: number[]
  width?: number
  height?: number
  color?: string
  fillColor?: string
}

export default function Sparkline({
  data,
  width = 200,
  height = 40,
  color = '#6366f1',
  fillColor = '#6366f140',
}: SparklineProps) {
  if (data.length < 2) return null

  const max = Math.max(...data, 1)
  const padY = 2

  const points = data.map((v, i) => {
    const x = (i / (data.length - 1)) * width
    const y = padY + (1 - v / max) * (height - padY * 2)
    return `${x},${y}`
  })

  const polyline = points.join(' ')
  const fillPath = `0,${height} ${polyline} ${width},${height}`

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className="w-full"
      style={{ maxHeight: height }}
      preserveAspectRatio="none"
    >
      <polygon points={fillPath} fill={fillColor} />
      <polyline
        points={polyline}
        fill="none"
        stroke={color}
        strokeWidth={1.5}
        strokeLinejoin="round"
        strokeLinecap="round"
      />
    </svg>
  )
}
