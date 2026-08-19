import "./globals.css";
import Nav from "../components/Nav";
import PwaRegister from "../components/PwaRegister";

export const metadata = {
  title: "Tropical Management",
  description: "Restaurant Management & Internal Audit",
  manifest: "/manifest.webmanifest",
};

export default function RootLayout({ children }) {
  return (
    <html lang="id">
      <body>
        <PwaRegister />
        <div className="tropical-shell">
          <Nav />
          {children}
        </div>
      </body>
    </html>
  );
}
