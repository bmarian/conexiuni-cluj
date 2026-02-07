import type {Metadata} from "next";
import "./globals.css";
import Navbar from "@/app/components/Navbar";

export const metadata: Metadata = {
  title: "Conexiuni Cluj",
  icons: {icon: "/favicon.svg"},
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode; }>) {
  return (
    <html lang="ro">
      <body>
        <Navbar />
        <main>{children}</main>
      </body>
    </html>
  );
}
