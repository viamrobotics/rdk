describe('should load the page', () => {
  it('passes', () => {
    cy.visit('/');

    cy.contains('h2', 'test_base').should('exist');
    cy.contains('h2', 'test_gantry').should('exist');
    cy.contains('h2', 'test_movement').should('exist');
    cy.contains('h2', 'test_arm').should('exist');
    cy.contains('h2', 'test_gripper').should('exist');
    cy.contains('h2', 'test_servo').should('exist');
    cy.contains('h2', 'test_motor_left').should('exist');
    cy.contains('h2', 'test_motor_right').should('exist');
    cy.contains('h2', 'test_input').should('exist');
    cy.contains('h2', 'WebGamepad').should('exist');
    cy.contains('h2', 'test_audio').should('exist');
    cy.contains('h2', 'test_board').should('exist');
    cy.contains('h2', 'test_camera').should('exist');
    cy.contains('h2', 'test_navigation').should('exist');
    cy.contains('h2', 'Sensors').should('exist');
    cy.contains('h2', 'Operations').should('exist');
    cy.contains('h2', 'DoCommand()').should('exist');
  });
});

export {};
