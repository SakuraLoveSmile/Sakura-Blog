import { ref, computed } from "vue";
import type { FixtureItem, FixtureOverrideConfig, FixtureState } from "./fixture-types";
import { FIXTURE_ITEMS } from "./fixture-definitions";
import { parseFixtureImport, type PersistedFixtureState } from "./fixture-import";

const STORAGE_KEY = "blog:console:mock_fixtures_v1";

function loadPersistedState(): PersistedFixtureState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { customData: {}, states: {} };
    const parsed = JSON.parse(raw);
    return {
      customData: parsed.customData || {},
      states: parsed.states || {},
    };
  } catch {
    return { customData: {}, states: {} };
  }
}

function savePersistedState(state: PersistedFixtureState) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch (err) {
    console.error("Failed to save mock fixtures state", err);
  }
}

const initial = loadPersistedState();
const selectedId = ref<string>(FIXTURE_ITEMS[0]?.id || "friends");
const customData = ref<Record<string, string>>(initial.customData);
const states = ref<Record<string, FixtureState>>(initial.states);

export function useFixtureStore() {
  const currentFixture = computed<FixtureItem>(
    () => FIXTURE_ITEMS.find((item) => item.id === selectedId.value) || FIXTURE_ITEMS[0],
  );

  const currentData = computed<string>(() => {
    const custom = customData.value[selectedId.value];
    if (custom !== undefined) return custom;
    return currentFixture.value.defaultData;
  });

  const currentState = computed<FixtureState>(() => {
    return states.value[selectedId.value] || "normal";
  });

  const isCustom = (id: string): boolean => {
    const custom = customData.value[id];
    const def = FIXTURE_ITEMS.find((f) => f.id === id)?.defaultData;
    return custom !== undefined && custom !== def;
  };

  const isModified = (id: string): boolean => {
    return isCustom(id) || (states.value[id] !== undefined && states.value[id] !== "normal");
  };

  const customCount = computed<number>(() => {
    let count = 0;
    for (const item of FIXTURE_ITEMS) {
      if (isModified(item.id)) count++;
    }
    return count;
  });

  const overrideMap = computed<Record<string, FixtureOverrideConfig>>(() => {
    const map: Record<string, FixtureOverrideConfig> = {};
    for (const item of FIXTURE_ITEMS) {
      map[item.id] = {
        endpoint: item.endpoint,
        format: item.format,
        state: states.value[item.id] || "normal",
        customData: customData.value[item.id] !== undefined ? customData.value[item.id] : item.defaultData,
        emptyData: item.emptyData,
      };
    }
    return map;
  });

  function selectFixture(id: string) {
    selectedId.value = id;
  }

  function updateData(id: string, data: string) {
    customData.value = { ...customData.value, [id]: data };
    savePersistedState({ customData: customData.value, states: states.value });
  }

  function updateState(id: string, state: FixtureState) {
    states.value = { ...states.value, [id]: state };
    savePersistedState({ customData: customData.value, states: states.value });
  }

  function resetFixture(id: string) {
    const nextCustom = { ...customData.value };
    delete nextCustom[id];
    const nextStates = { ...states.value };
    delete nextStates[id];
    customData.value = nextCustom;
    states.value = nextStates;
    savePersistedState({ customData: nextCustom, states: nextStates });
  }

  function resetAll() {
    customData.value = {};
    states.value = {};
    savePersistedState({ customData: {}, states: {} });
  }

  function exportJson(): string {
    return JSON.stringify(
      {
        version: 1,
        exportedAt: new Date().toISOString(),
        customData: customData.value,
        states: states.value,
      },
      null,
      2,
    );
  }

  function importJson(raw: string): { success: boolean; error?: string } {
    const result = parseFixtureImport(raw, FIXTURE_ITEMS);
    if (!result.success) return result;

    customData.value = result.state.customData;
    states.value = result.state.states;
    savePersistedState(result.state);
    return { success: true };
  }

  return {
    fixtures: FIXTURE_ITEMS,
    selectedId,
    currentFixture,
    currentData,
    currentState,
    customCount,
    overrideMap,
    isCustom,
    isModified,
    selectFixture,
    updateData,
    updateState,
    resetFixture,
    resetAll,
    exportJson,
    importJson,
  };
}
