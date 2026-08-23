import EnterpriseLoader from "../components/EnterpriseLoader";

export default function Loading() {
  return (
    <EnterpriseLoader
      embedded
      message="Memuat ruang kerja operasional"
      detail="Menyiapkan tampilan dan modul antarmuka terbaru"
    />
  );
}
