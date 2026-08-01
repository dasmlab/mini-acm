<template>
  <div class="box-view" role="img" aria-label="Single host machine box view: RHEL, libvirt, VyOS edge, mgmt SNO with ACM, and deployment clusters">
    <svg viewBox="0 0 920 560" xmlns="http://www.w3.org/2000/svg" class="box-svg">
      <defs>
        <marker id="arr" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto">
          <path d="M0,0 L7,3.5 L0,7 Z" fill="#607d8b" />
        </marker>
        <marker id="arr-teal" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto">
          <path d="M0,0 L7,3.5 L0,7 Z" fill="#00838f" />
        </marker>
        <marker id="arr-orange" markerWidth="7" markerHeight="7" refX="6" refY="3.5" orient="auto">
          <path d="M0,0 L7,3.5 L0,7 Z" fill="#ef6c00" />
        </marker>
        <pattern id="grid" width="20" height="20" patternUnits="userSpaceOnUse">
          <path d="M 20 0 L 0 0 0 20" fill="none" stroke="rgba(15,40,80,0.05)" stroke-width="1" />
        </pattern>
      </defs>

      <rect width="920" height="560" fill="url(#grid)" />

      <!-- Outer: single host -->
      <rect x="24" y="28" width="872" height="508" rx="18" fill="#fafbfd" stroke="#37474f" stroke-width="2.5" />
      <text x="44" y="54" class="t-host">SINGLE HOST MACHINE</text>
      <text x="260" y="54" class="t-muted">RHEL 10 · libvirt · nested guests</text>

      <!-- mini-mock -->
      <rect x="48" y="78" width="130" height="48" rx="8" fill="#0d47a1" />
      <text x="113" y="107" text-anchor="middle" class="t-on-dark">mini-mock</text>

      <!-- Orchestration label + line to mgmt -->
      <path d="M178 95 H 520" stroke="#00838f" stroke-width="1.75" fill="none" marker-end="url(#arr-teal)" />
      <text x="250" y="88" class="t-label-teal">oc / ACM orchestration</text>

      <!-- Provisioning link to libvirt -->
      <path d="M113 126 V 420" stroke="#546e7a" stroke-width="1.5" stroke-dasharray="5 3" fill="none" marker-end="url(#arr)" />
      <text x="120" y="280" class="t-label-grey" transform="rotate(-90 120 280)">IaaS provision → libvirt</text>

      <!-- Mgmt stack (orange dashed) -->
      <rect x="530" y="72" width="150" height="132" rx="10" fill="none" stroke="#ef6c00" stroke-width="2" stroke-dasharray="6 4" />
      <text x="605" y="90" text-anchor="middle" class="t-label-orange">MGMT</text>
      <rect x="548" y="98" width="114" height="28" rx="5" fill="#00838f" />
      <text x="605" y="117" text-anchor="middle" class="t-on-dark-sm">ACM</text>
      <rect x="548" y="130" width="114" height="28" rx="5" fill="#1a237e" />
      <text x="605" y="149" text-anchor="middle" class="t-on-dark-sm">OCP SNO</text>
      <rect x="548" y="162" width="114" height="28" rx="5" fill="#455a64" />
      <text x="605" y="181" text-anchor="middle" class="t-on-dark-sm">vHOST</text>

      <!-- Deployment group 1 -->
      <rect x="320" y="230" width="248" height="118" rx="10" fill="none" stroke="#1565c0" stroke-width="2" stroke-dasharray="6 4" />
      <text x="444" y="248" text-anchor="middle" class="t-label-blue">DEPLOYMENT-CLUSTER-1</text>
      <g v-for="(x, i) in [336, 412, 488]" :key="'d1'+i">
        <rect :x="x" y="258" width="64" height="26" rx="4" fill="#1565c0" />
        <text :x="x + 32" y="276" text-anchor="middle" class="t-on-dark-xs">OCP</text>
        <rect :x="x" y="288" width="64" height="26" rx="4" fill="#546e7a" />
        <text :x="x + 32" y="306" text-anchor="middle" class="t-on-dark-xs">vHOST</text>
      </g>

      <!-- Deployment group 2 -->
      <rect x="600" y="230" width="248" height="118" rx="10" fill="none" stroke="#1565c0" stroke-width="2" stroke-dasharray="6 4" />
      <text x="724" y="248" text-anchor="middle" class="t-label-blue">DEPLOYMENT-CLUSTER-2</text>
      <g v-for="(x, i) in [616, 692, 768]" :key="'d2'+i">
        <rect :x="x" y="258" width="64" height="26" rx="4" fill="#1565c0" />
        <text :x="x + 32" y="276" text-anchor="middle" class="t-on-dark-xs">OCP</text>
        <rect :x="x" y="288" width="64" height="26" rx="4" fill="#546e7a" />
        <text :x="x + 32" y="306" text-anchor="middle" class="t-on-dark-xs">vHOST</text>
      </g>

      <!-- ACM manages deployments -->
      <path d="M605 204 V 220 H 444 V 230" stroke="#00838f" stroke-width="1.5" fill="none" marker-end="url(#arr-teal)" />
      <path d="M605 204 V 220 H 724 V 230" stroke="#00838f" stroke-width="1.5" fill="none" marker-end="url(#arr-teal)" />
      <text x="660" y="216" class="t-label-teal">ACM manages LC</text>

      <!-- Lab VLAN bus -->
      <rect x="320" y="372" width="400" height="28" rx="6" fill="#eceff1" stroke="#78909c" stroke-width="1.5" />
      <text x="520" y="391" text-anchor="middle" class="t-label-grey">LAB LAN · 10.77.30.0/24 (libvirt)</text>

      <!-- Guests drop to VLAN -->
      <path d="M605 194 V 204" stroke="#ef6c00" stroke-width="1.25" fill="none" />
      <path d="M548 190 H 400 V 372" stroke="#78909c" stroke-width="1.25" stroke-dasharray="3 2" fill="none" marker-end="url(#arr)" />
      <path d="M444 348 V 372" stroke="#78909c" stroke-width="1.25" stroke-dasharray="3 2" fill="none" marker-end="url(#arr)" />
      <path d="M724 348 V 372" stroke="#78909c" stroke-width="1.25" stroke-dasharray="3 2" fill="none" marker-end="url(#arr)" />

      <!-- VyOS / vRouter -->
      <ellipse cx="200" cy="386" rx="70" ry="32" fill="#fff3e0" stroke="#e65100" stroke-width="2" />
      <text x="200" y="382" text-anchor="middle" class="t-label-orange">VYOS-GW</text>
      <text x="200" y="398" text-anchor="middle" class="t-muted-xs">vRouter · NAT/FW</text>
      <path d="M320 386 H 270" stroke="#e65100" stroke-width="1.75" fill="none" marker-end="url(#arr-orange)" />
      <text x="285" y="378" class="t-label-orange">LAN</text>

      <!-- vWAN -->
      <rect x="48" y="368" width="72" height="36" rx="6" fill="#37474f" />
      <text x="84" y="391" text-anchor="middle" class="t-on-dark-sm">vWAN</text>
      <path d="M130 386 H 130" stroke="#e65100" stroke-width="1.5" fill="none" />
      <path d="M130 386 H 148" stroke="#e65100" stroke-width="1.75" fill="none" marker-end="url(#arr-orange)" />
      <text x="138" y="378" class="t-label-orange">WAN</text>

      <!-- Libvirt -->
      <rect x="200" y="430" width="520" height="36" rx="7" fill="#455a64" />
      <text x="460" y="453" text-anchor="middle" class="t-on-dark">LIBVIRT</text>
      <path d="M605 190 V 210" stroke="transparent" />
      <path d="M368 318 V 430" stroke="#546e7a" stroke-width="1.25" stroke-dasharray="4 3" fill="none" marker-end="url(#arr)" />
      <path d="M444 318 V 430" stroke="#546e7a" stroke-width="1.25" stroke-dasharray="4 3" fill="none" marker-end="url(#arr)" />
      <path d="M520 318 V 430" stroke="#546e7a" stroke-width="1.25" stroke-dasharray="4 3" fill="none" marker-end="url(#arr)" />
      <path d="M648 318 V 430" stroke="#546e7a" stroke-width="1.25" stroke-dasharray="4 3" fill="none" marker-end="url(#arr)" />
      <path d="M724 318 V 430" stroke="#546e7a" stroke-width="1.25" stroke-dasharray="4 3" fill="none" marker-end="url(#arr)" />
      <path d="M800 318 V 430" stroke="#546e7a" stroke-width="1.25" stroke-dasharray="4 3" fill="none" marker-end="url(#arr)" />
      <path d="M605 190 V 430" stroke="#546e7a" stroke-width="1.5" stroke-dasharray="4 3" fill="none" marker-end="url(#arr)" />

      <!-- RHEL + hardware -->
      <rect x="48" y="480" width="680" height="28" rx="6" fill="#1b5e20" />
      <text x="388" y="499" text-anchor="middle" class="t-on-dark-sm">RHEL 10</text>
      <rect x="48" y="512" width="680" height="14" rx="4" fill="#263238" />
      <text x="388" y="523" text-anchor="middle" class="t-on-dark-xs">CPU / RAM / DISK</text>

      <!-- NICs -->
      <polygon points="760,498 780,488 800,498 780,508" fill="#ff8f00" stroke="#e65100" stroke-width="1" />
      <text x="780" y="528" text-anchor="middle" class="t-muted-xs">NIC</text>
      <text x="780" y="478" text-anchor="middle" class="t-label-grey">bridged</text>
      <path d="M120 404 V 480" stroke="#e65100" stroke-width="1.5" fill="none" marker-end="url(#arr-orange)" />

      <polygon points="840,498 860,488 880,498 860,508" fill="#90a4ae" stroke="#546e7a" stroke-width="1" />
      <text x="860" y="528" text-anchor="middle" class="t-muted-xs">NIC</text>
      <text x="860" y="478" text-anchor="middle" class="t-label-grey">host-only</text>

      <!-- External cloud -->
      <ellipse cx="780" cy="430" rx="48" ry="22" fill="#e3f2fd" stroke="#1565c0" stroke-width="1.5" />
      <text x="780" y="435" text-anchor="middle" class="t-label-blue">external</text>
      <path d="M780 452 V 488" stroke="#1565c0" stroke-width="1.5" fill="none" marker-end="url(#arr)" />

      <!-- Legend -->
      <text x="760" y="78" class="t-muted-xs">orange = runs ACM</text>
      <text x="760" y="94" class="t-muted-xs">blue = managed by ACM</text>
    </svg>
  </div>
</template>

<script setup>
</script>

<style scoped>
.box-view {
  width: 100%;
  border: 1px solid #e0e0e0;
  border-radius: 12px;
  background: #fff;
  overflow: hidden;
}
.box-svg {
  display: block;
  width: 100%;
  height: auto;
}
.t-host {
  fill: #263238;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.04em;
}
.t-muted {
  fill: #90a4ae;
  font-size: 11px;
}
.t-muted-xs {
  fill: #90a4ae;
  font-size: 9px;
}
.t-on-dark {
  fill: #fff;
  font-size: 14px;
  font-weight: 700;
}
.t-on-dark-sm {
  fill: #fff;
  font-size: 11px;
  font-weight: 600;
}
.t-on-dark-xs {
  fill: #fff;
  font-size: 10px;
  font-weight: 600;
}
.t-label-teal {
  fill: #00838f;
  font-size: 10px;
  font-weight: 600;
}
.t-label-orange {
  fill: #e65100;
  font-size: 10px;
  font-weight: 600;
}
.t-label-blue {
  fill: #1565c0;
  font-size: 10px;
  font-weight: 600;
}
.t-label-grey {
  fill: #607d8b;
  font-size: 10px;
  font-weight: 500;
}
</style>
