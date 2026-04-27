<!-- Bus side-view illustration used on the 404 page.
     Uses currentColor throughout so it inherits the container's text color
     and adapts to any theme automatically. -->
<template>
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 200 96"
    fill="none"
    aria-hidden="true"
  >
    <!-- ground shadow -->
    <ellipse cx="100" cy="93" rx="72" ry="4" fill="currentColor" opacity="0.08"/>

    <!-- destination / route sign at top -->
    <rect x="16" y="2" width="100" height="20" rx="4" fill="currentColor" opacity="0.90"/>
    <text
      x="66" y="15.5"
      text-anchor="middle"
      font-size="9"
      font-weight="700"
      font-family="ui-monospace, 'Courier New', monospace"
      fill="white"
      letter-spacing="0.5"
    >404 · NOT FOUND
    </text>

    <!-- bus body fill -->
    <rect x="6" y="20" width="188" height="58" rx="12" fill="currentColor" opacity="0.10"/>
    <!-- bus body outline -->
    <rect x="6" y="20" width="188" height="58" rx="12" stroke="currentColor" stroke-width="2.5"/>

    <!-- windows (4 equal) -->
    <rect x="18" y="28" width="30" height="20" rx="5" fill="currentColor" opacity="0.22"/>
    <rect x="56" y="28" width="30" height="20" rx="5" fill="currentColor" opacity="0.22"/>
    <rect x="94" y="28" width="30" height="20" rx="5" fill="currentColor" opacity="0.22"/>
    <rect x="132" y="28" width="30" height="20" rx="5" fill="currentColor" opacity="0.22"/>

    <!-- door (second section, open/closed line down centre) -->
    <rect x="60" y="52" width="22" height="24" rx="3" fill="none" stroke="currentColor"
          stroke-width="1.8"/>
    <line x1="71" y1="52" x2="71" y2="76" stroke="currentColor" stroke-width="1.8"/>
    <!-- door handle dot -->
    <circle cx="65" cy="64" r="1.6" fill="currentColor" opacity="0.7"/>

    <!-- headlight -->
    <rect x="187" y="28" width="6" height="12" rx="3" fill="currentColor" opacity="0.45"/>

    <!-- wheel wells (arch cutouts implied by the undercarriage line) -->
    <rect x="20" y="71" width="136" height="6" rx="3" fill="currentColor" opacity="0.18"/>

    <!-- left wheel -->
    <circle cx="45" cy="80" r="13" fill="currentColor" opacity="0.85"/>
    <circle cx="45" cy="80" r="7" fill="currentColor" opacity="0.20"/>
    <circle cx="45" cy="80" r="3" fill="currentColor" opacity="0.70"/>

    <!-- right wheel -->
    <circle cx="155" cy="80" r="13" fill="currentColor" opacity="0.85"/>
    <circle cx="155" cy="80" r="7" fill="currentColor" opacity="0.20"/>
    <circle cx="155" cy="80" r="3" fill="currentColor" opacity="0.70"/>

    <!-- X marks on the wheels (wrong direction / out of service) -->
    <line x1="39" y1="74" x2="51" y2="86" stroke="white" stroke-width="1.5" opacity="0.55"/>
    <line x1="51" y1="74" x2="39" y2="86" stroke="white" stroke-width="1.5" opacity="0.55"/>
    <line x1="149" y1="74" x2="161" y2="86" stroke="white" stroke-width="1.5" opacity="0.55"/>
    <line x1="161" y1="74" x2="149" y2="86" stroke="white" stroke-width="1.5" opacity="0.55"/>
  </svg>
</template>

