import {storiesOf} from "@storybook/vue";
import {withDesign} from 'storybook-addon-designs';
import VueI18n from 'vue-i18n';

function loadLocaleMessages() {
    const locales = require.context("../i18n", true, /[A-Za-z0-9-_,\s]+\.json$/i);
    const messages = {};
    locales.keys().forEach((key) => {
        const matched = key.match(/([A-Za-z0-9-_]+)\./i);
        if (matched && matched.length > 1) {
            const locale = matched[1];
            messages[locale] = locales(key);
        }
    });
    return messages;
}

storiesOf("MotorDetail", module).add("Default MotorDetail", () => ({
    i18n: new VueI18n({
        locale: 'en',
        fallbackLocale: 'en',
        messages: loadLocaleMessages(),
    }),
    data() {
        return {
            status: {
                on: false,
                positionSupported: true,
                position: 0,
                pidConfig: {
                    fieldsMap: [
                        ['Kd', {numberValue: 0}],
                        ['Kp', {numberValue: 0.34}],
                        ['Ki', {numberValue: 0.77}],
                    ],
                },
            },
        };
    },
    template:
        '<MotorDetail motorName="test" :motorStatus="status"></MotorDetail>',
}));
