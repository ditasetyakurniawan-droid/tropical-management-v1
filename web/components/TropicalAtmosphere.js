export default function TropicalAtmosphere() {
  const leaves = [
    ["leaf-a", "M22 3C9 7 3 18 5 31c12 1 24-6 27-19-1 10-7 17-18 21 11 1 20-4 24-12C43 9 34 2 22 3Z"],
    ["leaf-b", "M8 36C7 20 16 8 31 4c5 14 0 27-12 34 4-9 5-17 3-25-1 10-6 18-14 23Z"],
    ["leaf-c", "M4 21C11 8 23 3 37 7c-1 15-10 25-25 28 8-6 13-13 16-22-6 9-14 14-24 15Z"],
  ];

  return (
    <div className="botanical-atmosphere" aria-hidden="true">
      <div className="botanical-glow botanical-glow-one" />
      <div className="botanical-glow botanical-glow-two" />
      {Array.from({ length: 11 }).map((_, index) => {
        const [variant, path] = leaves[index % leaves.length];
        return (
          <svg key={index} className={`floating-leaf ${variant} leaf-${index + 1}`} viewBox="0 0 44 44" role="presentation">
            <path d={path} />
            <path className="leaf-vein" d="M10 34C18 26 24 18 31 8" />
          </svg>
        );
      })}
    </div>
  );
}
