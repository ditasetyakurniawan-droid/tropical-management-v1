import "./globals.css";
import "./typography.css";
import Nav from "../components/Nav";
import PwaRegister from "../components/PwaRegister";
import SessionGate from "../components/SessionGate";
import TropicalAtmosphere from "../components/TropicalAtmosphere";
import RouteTransition from "../components/RouteTransition";
import LiveChatProvider from "../components/LiveChatProvider";
import FloatingChat from "../components/FloatingChat";

export const metadata = {
  title: "Tropical Steak House · Internal OS",
  description: "Ruang kerja internal Tropical Steak House untuk people, shift, kualitas, stok, penjualan, dan koordinasi operasional.",
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
          <LiveChatProvider>
            <SessionGate>
              <div className="relative z-10">
                <RouteTransition>{children}</RouteTransition>
              </div>
            </SessionGate>
            <FloatingChat />
          </LiveChatProvider>
        </div>
      </body>
    </html>
  );
}
