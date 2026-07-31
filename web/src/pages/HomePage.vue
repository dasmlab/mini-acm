<template>
  <q-page padding>
    <div class="text-h4 q-mb-sm">mini-acm</div>
    <p class="text-body1 text-grey-8 q-mb-lg">
      Design a lab rack as a <strong>MockUp</strong>, edit the topology
      (MGMT-CLUSTER → ACM → DEPLOYMENT-CLUSTER), capture sizing and MVP-gap
      parameters, then derive CLI YAML and run the install steps.
    </p>

    <q-banner class="bg-blue-1 text-primary q-mb-xl" rounded>
      <template #avatar><q-icon name="science" color="primary" /></template>
      LAB / TEST / DEV ONLY. Secrets stay in env / file paths — never in MockUp YAML in git.
    </q-banner>

    <div class="row q-col-gutter-md">
      <div class="col-12 col-sm-4">
        <q-card flat bordered class="cursor-pointer action-card" @click="$router.push({ name: 'mockups' })">
          <q-card-section>
            <div class="text-h6"><q-icon name="account_tree" class="q-mr-sm" />MockUps</div>
            <div class="text-caption text-grey-7">Create / open a lab rack blueprint</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-12 col-sm-4">
        <q-card flat bordered class="cursor-pointer action-card" @click="$router.push({ name: 'mockups' })">
          <q-card-section>
            <div class="text-h6"><q-icon name="schema" class="q-mr-sm" />Topology</div>
            <div class="text-caption text-grey-7">Drag nodes, connect, edit hub / ACM / clusters</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-12 col-sm-4">
        <q-card flat bordered class="cursor-pointer action-card" @click="$router.push({ name: 'mockups' })">
          <q-card-section>
            <div class="text-h6"><q-icon name="playlist_play" class="q-mr-sm" />Wizard</div>
            <div class="text-caption text-grey-7">Steps: hub → ACM → cluster → attach-iso</div>
          </q-card-section>
        </q-card>
      </div>
    </div>
  </q-page>
</template>

<script setup>
</script>
