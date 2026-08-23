import "./globals.css";
import "./typography.css";
import Nav from "../components/Nav";
import PwaRegister from "../components/PwaRegister";
import SessionGate from "../components/SessionGate";
import TropicalAtmosphere from "../components/TropicalAtmosphere";
import RouteTransition from "../components/RouteTransition";

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
            <div className="relative z-10">
              <RouteTransition>{children}</RouteTransition>
            </div>
          </SessionGate>
        </div>
      </body>
    </html>
  );
}
