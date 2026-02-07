"use client";

import {useEffect, useState} from "react";
import {
  FavoriteLine,
  isLineFavorite,
  isStopFavorite,
  toggleFavoriteLine,
  toggleFavoriteStop,
} from "@/lib/favorites";
import {RouteType} from "@/types/tranzy";

function StarIcon({filled}: { filled: boolean }) {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill={filled ? "currentColor" : "none"}
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
    </svg>
  );
}

export function FavoriteLineButton({
  routeShortName,
  routeLongName,
  routeColor,
  routeType,
}: {
  routeShortName: string;
  routeLongName: string;
  routeColor: string;
  routeType: RouteType;
}) {
  const [isFav, setIsFav] = useState(false);

  useEffect(() => {
    setIsFav(isLineFavorite(routeShortName));
  }, [routeShortName]);

  const handleClick = () => {
    const line: FavoriteLine = {
      route_short_name: routeShortName,
      route_long_name: routeLongName,
      route_color: routeColor,
      route_type: routeType,
    };
    const nowFav = toggleFavoriteLine(line);
    setIsFav(nowFav);
  };

  return (
    <button
      onClick={handleClick}
      className={`shrink-0 rounded-lg border p-2 transition-colors ${
        isFav
          ? "border-yellow-300 bg-yellow-50 text-yellow-500 dark:border-yellow-700 dark:bg-yellow-950/30 dark:text-yellow-400"
          : "border-zinc-200 text-zinc-400 hover:border-zinc-300 hover:text-zinc-500 dark:border-zinc-700 dark:hover:border-zinc-600 dark:hover:text-zinc-300"
      }`}
      title={isFav ? "Elimină de la favorite" : "Adaugă la favorite"}
    >
      <StarIcon filled={isFav} />
    </button>
  );
}

export function FavoriteStopButton({stopName}: { stopName: string }) {
  const [isFav, setIsFav] = useState(false);

  useEffect(() => {
    setIsFav(isStopFavorite(stopName));
  }, [stopName]);

  const handleClick = () => {
    const nowFav = toggleFavoriteStop(stopName);
    setIsFav(nowFav);
  };

  return (
    <button
      onClick={handleClick}
      className={`shrink-0 rounded-lg border p-2 transition-colors ${
        isFav
          ? "border-yellow-300 bg-yellow-50 text-yellow-500 dark:border-yellow-700 dark:bg-yellow-950/30 dark:text-yellow-400"
          : "border-zinc-200 text-zinc-400 hover:border-zinc-300 hover:text-zinc-500 dark:border-zinc-700 dark:hover:border-zinc-600 dark:hover:text-zinc-300"
      }`}
      title={isFav ? "Elimină de la favorite" : "Adaugă la favorite"}
    >
      <StarIcon filled={isFav} />
    </button>
  );
}
