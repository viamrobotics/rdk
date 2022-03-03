import {storiesOf} from "@storybook/vue";
import {withDesign} from 'storybook-addon-designs';

storiesOf("MotorDetail", module).add("Default MotorDetail", () => ({
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
