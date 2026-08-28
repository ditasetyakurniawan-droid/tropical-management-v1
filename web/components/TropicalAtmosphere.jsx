// Konstanta di luar komponen agar tidak dibuat ulang setiap render
const LEAF_COUNT = 11;
const LEAF_VARIANTS = [
  {
    variant: "leaf-a",
    path: "M22 3C9 7 3 18 5 31c12 1 24-6 27-19-1 10-7 17-18 21 11 1 20-4 24-12C43 9 34 2 22 3Z",
  },
  {
    variant: "leaf-b",
    path: "M8 36C7 20 16 8 31 4c5 14 0 27-12 34 4-9 5-17 3-25-1 10-6 18-14 23Z",
  },
  {
    variant: "leaf-c",
    path: "M4 21C11 8 23 3 37 7c-1 15-10 25-25 28 8-6 13-13 16-22-6 9-14 14-24 15Z",
  },
];
const LEAF_VEIN_PATH = "M10 34C18 26 24 18 31 8";

// Data daun statis dengan id unik untuk key (bukan index array)
const LEAVES = Array.from({ length: LEAF_COUNT }, (_, index) => {
  const { variant, path } = LEAF_VARIANTS[index % LEAF_VARIANTS.length];
  return {
    id: `leaf-${index + 1}`,
    variant,
    path,
  };
});

export default function TropicalAtmosphere() {
  return (
    <div className="botanical-atmosphere" aria-hidden="true">
      <div className="botanical-glow botanical-glow-one" />
      <div className="botanical-glow botanical-glow-two" />

      {LEAVES.map((leaf) => (
        <svg
          key={leaf.id}
          className={`floating-leaf ${leaf.variant} ${leaf.id}`}
          viewBox="0 0 44 44"
          role="presentation"
        >
          <path d={leaf.path} />
          <path className="leaf-vein" d={LEAF_VEIN_PATH} />
        </svg>
      ))}
    </div>
  );
}