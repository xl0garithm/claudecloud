"use client";

interface PricingCardProps {
  name: string;
  price: string;
  period: string;
  features: string[];
  cta: string;
  highlighted?: boolean;
  badge?: string;
  onSelect: () => void;
}

export default function PricingCard({
  name,
  price,
  period,
  features,
  cta,
  highlighted,
  badge,
  onSelect,
}: PricingCardProps) {
  return (
    <div
      className={`relative rounded-2xl p-8 ${
        highlighted
          ? "bg-gray-900 text-white ring-2 ring-gray-900"
          : "bg-white ring-1 ring-gray-200"
      }`}
    >
      {badge && (
        <span className="absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-green-500 px-3 py-1 text-xs font-semibold text-white">
          {badge}
        </span>
      )}
      <h3 className="text-lg font-semibold">{name}</h3>
      <div className="mt-4 flex items-baseline gap-1">
        <span className="text-4xl font-bold">{price}</span>
        <span className={highlighted ? "text-gray-400" : "text-gray-500"}>
          {period}
        </span>
      </div>
      <ul className="mt-6 space-y-3">
        {features.map((feature) => (
          <li key={feature} className="flex items-start gap-2">
            <span className={highlighted ? "text-green-400" : "text-green-600"}>
              &#10003;
            </span>
            <span className={highlighted ? "text-gray-300" : "text-gray-600"}>
              {feature}
            </span>
          </li>
        ))}
      </ul>
      <button
        onClick={onSelect}
        className={`mt-8 w-full rounded-lg py-3 font-medium transition ${
          highlighted
            ? "bg-white text-gray-900 hover:bg-gray-100"
            : "bg-gray-900 text-white hover:bg-gray-800"
        }`}
      >
        {cta}
      </button>
    </div>
  );
}
