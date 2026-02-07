import type {Metadata} from "next";
import "./globals.css";
import Navbar from "@/app/components/Navbar";
import LeafletPreloader from "@/app/components/LeafletPreloader";
import NavigationProgress from "@/app/components/NavigationProgress";

export const metadata: Metadata = {
  title: "Conexiuni Cluj",
  icons: {icon: "/favicon.svg"},
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode; }>) {
  return (
    <html lang="ro">
      <body>
        <NavigationProgress />
        <Navbar />
        <LeafletPreloader />
        <main>{children}</main>
      </body>
    </html>
  );
}
