import "./globals.css";
import Nav from "../components/Nav";
import PwaRegister from "../components/PwaRegister";
import SessionGate from "../components/SessionGate";
import TropicalAtmosphere from "../components/TropicalAtmosphere";

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
          <TropicalAtmosphere />
          <Nav />
          <SessionGate>
            <div className="relative z-10">{children}</div>
          </SessionGate>
        </div>
      </body>
    </html>
  );
}
